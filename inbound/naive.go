package inbound

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
)

var _ adapter.Inbound = (*Naive)(nil)

type Naive struct {
	myInboundAdapter
	authenticator auth.Authenticator
	tlsConfig     *TLSConfig
	httpServer    *http.Server
	h3Server      any
}

func NewNaive(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.NaiveInboundOptions) (*Naive, error) {
	inbound := &Naive{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeNaive,
			network:       options.Network.Build(),
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		authenticator: auth.NewAuthenticator(options.Users),
	}
	if common.Contains(inbound.network, N.NetworkUDP) {
		if options.TLS == nil || !options.TLS.Enabled {
			return nil, E.New("TLS is required for QUIC server")
		}
	}
	if len(options.Users) == 0 {
		return nil, E.New("missing users")
	}
	if options.TLS != nil {
		tlsConfig, err := NewTLSConfig(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
		inbound.tlsConfig = tlsConfig
	}
	return inbound, nil
}

func (n *Naive) Start() error {
	var tlsConfig *tls.Config
	if n.tlsConfig != nil {
		err := n.tlsConfig.Start()
		if err != nil {
			return E.Cause(err, "create TLS config")
		}
		tlsConfig = n.tlsConfig.Config()
	}

	if common.Contains(n.network, N.NetworkTCP) {
		tcpListener, err := n.ListenTCP()
		if err != nil {
			return err
		}
		n.httpServer = &http.Server{
			Handler:   n,
			TLSConfig: tlsConfig,
		}
		go func() {
			var sErr error
			if tlsConfig != nil {
				sErr = n.httpServer.ServeTLS(tcpListener, "", "")
			} else {
				sErr = n.httpServer.Serve(tcpListener)
			}
			if sErr != nil && !E.IsClosedOrCanceled(sErr) {
				n.logger.Error("http server serve error: ", sErr)
			}
		}()
	}

	if common.Contains(n.network, N.NetworkUDP) {
		err := n.configureHTTP3Listener()
		if !C.QUIC_AVAILABLE && len(n.network) > 1 {
			log.Warn(E.Cause(err, "naive http3 disabled"))
		} else if err != nil {
			return err
		}
	}

	return nil
}

func (n *Naive) Close() error {
	return common.Close(
		&n.myInboundAdapter,
		common.PtrOrNil(n.httpServer),
		n.h3Server,
		common.PtrOrNil(n.tlsConfig),
	)
}

func (n *Naive) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx := log.ContextWithNewID(request.Context())
	if request.Method != "CONNECT" {
		rejectHTTP(writer, http.StatusBadRequest)
		n.badRequest(ctx, request, E.New("not CONNECT request"))
		return
	} else if request.Header.Get("Padding") == "" {
		rejectHTTP(writer, http.StatusBadRequest)
		n.badRequest(ctx, request, E.New("missing naive padding"))
		return
	}
	var authOk bool
	authorization := request.Header.Get("Proxy-Authorization")
	if strings.HasPrefix(authorization, "BASIC ") || strings.HasPrefix(authorization, "Basic ") {
		userPassword, _ := base64.URLEncoding.DecodeString(authorization[6:])
		userPswdArr := strings.SplitN(string(userPassword), ":", 2)
		authOk = n.authenticator.Verify(userPswdArr[0], userPswdArr[1])
		if authOk {
			ctx = auth.ContextWithUser(ctx, userPswdArr[0])
		}
	}
	if !authOk {
		rejectHTTP(writer, http.StatusProxyAuthRequired)
		n.badRequest(ctx, request, E.New("authorization failed"))
		return
	}
	writer.Header().Set("Padding", generateNaivePaddingHeader())
	writer.WriteHeader(http.StatusOK)
	writer.(http.Flusher).Flush()

	hostPort := request.URL.Host
	if hostPort == "" {
		hostPort = request.Host
	}
	source := M.ParseSocksaddr(request.RemoteAddr)
	destination := M.ParseSocksaddr(hostPort)

	if hijacker, isHijacker := writer.(http.Hijacker); isHijacker {
		conn, _, err := hijacker.Hijack()
		if err != nil {
			return
		}
		n.newConnection(ctx, &naiveH1Conn{Conn: conn}, source, destination)
	} else {
		n.newConnection(ctx, &naiveH2Conn{reader: request.Body, writer: writer, flusher: writer.(http.Flusher)}, source, destination)
	}
}

func (n *Naive) newConnection(ctx context.Context, conn net.Conn, source, destination M.Socksaddr) {
	metadata := n.createMetadata(conn)
	metadata.Source = source
	metadata.Destination = destination
	n.routeTCP(ctx, conn, metadata)
}

func (n *Naive) badRequest(ctx context.Context, request *http.Request, err error) {
	n.NewError(ctx, E.Cause(err, "process connection from ", request.RemoteAddr))
}

func rejectHTTP(writer http.ResponseWriter, statusCode int) {
	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		writer.WriteHeader(statusCode)
		return
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		writer.WriteHeader(statusCode)
		return
	}
	if tcpConn, isTCP := common.Cast[*net.TCPConn](conn); isTCP {
		tcpConn.SetLinger(0)
	}
	conn.Close()
}

func generateNaivePaddingHeader() string {
	paddingLen := rand.Intn(32) + 30
	padding := make([]byte, paddingLen)
	bits := rand.Uint64()
	for i := 0; i < 16; i++ {
		// Codes that won't be Huffman coded.
		padding[i] = "!#$()+<>?@[]^`{}"[bits&15]
		bits >>= 4
	}
	for i := 16; i < paddingLen; i++ {
		padding[i] = '~'
	}
	return string(padding)
}

const kFirstPaddings = 8

type naiveH1Conn struct {
	net.Conn
	readPadding      int
	writePadding     int
	readRemaining    int
	paddingRemaining int
}

func (c *naiveH1Conn) Read(p []byte) (n int, err error) {
	n, err = c.read(p)
	return n, wrapHttpError(err)
}

func (c *naiveH1Conn) read(p []byte) (n int, err error) {
	if c.readRemaining > 0 {
		if len(p) > c.readRemaining {
			p = p[:c.readRemaining]
		}
		n, err = c.Conn.Read(p)
		if err != nil {
			return
		}
		c.readRemaining -= n
		return
	}
	if c.paddingRemaining > 0 {
		err = rw.SkipN(c.Conn, c.paddingRemaining)
		if err != nil {
			return
		}
		c.readRemaining = 0
	}
	if c.readPadding < kFirstPaddings {
		paddingHdr := p[:3]
		_, err = io.ReadFull(c.Conn, paddingHdr)
		if err != nil {
			return
		}
		originalDataSize := int(binary.BigEndian.Uint16(paddingHdr[:2]))
		paddingSize := int(paddingHdr[2])
		if len(p) > originalDataSize {
			p = p[:originalDataSize]
		}
		n, err = c.Conn.Read(p)
		if err != nil {
			return
		}
		c.readPadding++
		c.readRemaining = originalDataSize - n
		c.paddingRemaining = paddingSize
		return
	}
	return c.Conn.Read(p)
}

func (c *naiveH1Conn) Write(p []byte) (n int, err error) {
	for pLen := len(p); pLen > 0; {
		var data []byte
		if pLen > 65535 {
			data = p[:65535]
			p = p[65535:]
			pLen -= 65535
		} else {
			data = p
			pLen = 0
		}
		var writeN int
		writeN, err = c.write(data)
		n += writeN
		if err != nil {
			break
		}
	}
	return n, wrapHttpError(err)
}

func (c *naiveH1Conn) write(p []byte) (n int, err error) {
	if c.writePadding < kFirstPaddings {
		paddingSize := rand.Intn(256)

		_buffer := buf.StackNewSize(3 + len(p) + paddingSize)
		defer common.KeepAlive(_buffer)
		buffer := common.Dup(_buffer)
		defer buffer.Release()
		header := buffer.Extend(3)
		binary.BigEndian.PutUint16(header, uint16(len(p)))
		header[2] = byte(paddingSize)

		common.Must1(buffer.Write(p))
		_, err = c.Conn.Write(buffer.Bytes())
		if err == nil {
			n = len(p)
		}
		c.writePadding++
		return
	}
	return c.Conn.Write(p)
}

func (c *naiveH1Conn) FrontHeadroom() int {
	if c.writePadding < kFirstPaddings {
		return 3
	}
	return 0
}

func (c *naiveH1Conn) RearHeadroom() int {
	if c.writePadding < kFirstPaddings {
		return 255
	}
	return 0
}

func (c *naiveH1Conn) WriterMTU() int {
	if c.writePadding < kFirstPaddings {
		return 65535
	}
	return 0
}

func (c *naiveH1Conn) WriteBuffer(buffer *buf.Buffer) error {
	defer buffer.Release()
	if c.writePadding < kFirstPaddings {
		bufferLen := buffer.Len()
		if bufferLen > 65535 {
			return common.Error(c.Write(buffer.Bytes()))
		}
		paddingSize := rand.Intn(256)
		header := buffer.ExtendHeader(3)
		binary.BigEndian.PutUint16(header, uint16(bufferLen))
		header[2] = byte(paddingSize)
		buffer.Extend(paddingSize)
		c.writePadding++
	}
	return wrapHttpError(common.Error(c.Conn.Write(buffer.Bytes())))
}

func (c *naiveH1Conn) WriteTo(w io.Writer) (n int64, err error) {
	if c.readPadding < kFirstPaddings {
		n, err = bufio.WriteToN(c, w, kFirstPaddings-c.readPadding)
	} else {
		n, err = bufio.Copy(w, c.Conn)
	}
	return n, wrapHttpError(err)
}

func (c *naiveH1Conn) ReadFrom(r io.Reader) (n int64, err error) {
	if c.writePadding < kFirstPaddings {
		n, err = bufio.ReadFromN(c, r, kFirstPaddings-c.writePadding)
	} else {
		n, err = bufio.Copy(c.Conn, r)
	}
	return n, wrapHttpError(err)
}

func (c *naiveH1Conn) Upstream() any {
	return c.Conn
}

func (c *naiveH1Conn) ReaderReplaceable() bool {
	return c.readRemaining == kFirstPaddings
}

func (c *naiveH1Conn) WriterReplaceable() bool {
	return c.writePadding == kFirstPaddings
}

type naiveH2Conn struct {
	reader           io.Reader
	writer           io.Writer
	flusher          http.Flusher
	rAddr            net.Addr
	readPadding      int
	writePadding     int
	readRemaining    int
	paddingRemaining int
}

func (c *naiveH2Conn) Read(p []byte) (n int, err error) {
	n, err = c.read(p)
	return n, wrapHttpError(err)
}

func (c *naiveH2Conn) read(p []byte) (n int, err error) {
	if c.readRemaining > 0 {
		if len(p) > c.readRemaining {
			p = p[:c.readRemaining]
		}
		n, err = c.reader.Read(p)
		if err != nil {
			return
		}
		c.readRemaining -= n
		return
	}
	if c.paddingRemaining > 0 {
		err = rw.SkipN(c.reader, c.paddingRemaining)
		if err != nil {
			return
		}
		c.readRemaining = 0
	}
	if c.readPadding < kFirstPaddings {
		paddingHdr := p[:3]
		_, err = io.ReadFull(c.reader, paddingHdr)
		if err != nil {
			return
		}
		originalDataSize := int(binary.BigEndian.Uint16(paddingHdr[:2]))
		paddingSize := int(paddingHdr[2])
		if len(p) > originalDataSize {
			p = p[:originalDataSize]
		}
		n, err = c.reader.Read(p)
		if err != nil {
			return
		}
		c.readPadding++
		c.readRemaining = originalDataSize - n
		c.paddingRemaining = paddingSize
		return
	}
	return c.reader.Read(p)
}

func (c *naiveH2Conn) Write(p []byte) (n int, err error) {
	for pLen := len(p); pLen > 0; {
		var data []byte
		if pLen > 65535 {
			data = p[:65535]
			p = p[65535:]
			pLen -= 65535
		} else {
			data = p
			pLen = 0
		}
		var writeN int
		writeN, err = c.write(data)
		n += writeN
		if err != nil {
			break
		}
	}
	if err == nil {
		c.flusher.Flush()
	}
	return n, wrapHttpError(err)
}

func (c *naiveH2Conn) write(p []byte) (n int, err error) {
	if c.writePadding < kFirstPaddings {
		paddingSize := rand.Intn(256)

		_buffer := buf.StackNewSize(3 + len(p) + paddingSize)
		defer common.KeepAlive(_buffer)
		buffer := common.Dup(_buffer)
		defer buffer.Release()
		header := buffer.Extend(3)
		binary.BigEndian.PutUint16(header, uint16(len(p)))
		header[2] = byte(paddingSize)

		common.Must1(buffer.Write(p))
		_, err = c.writer.Write(buffer.Bytes())
		if err == nil {
			n = len(p)
		}
		c.writePadding++
		return
	}
	return c.writer.Write(p)
}

func (c *naiveH2Conn) FrontHeadroom() int {
	if c.writePadding < kFirstPaddings {
		return 3
	}
	return 0
}

func (c *naiveH2Conn) RearHeadroom() int {
	if c.writePadding < kFirstPaddings {
		return 255
	}
	return 0
}

func (c *naiveH2Conn) WriterMTU() int {
	if c.writePadding < kFirstPaddings {
		return 65535
	}
	return 0
}

func (c *naiveH2Conn) WriteBuffer(buffer *buf.Buffer) error {
	defer buffer.Release()
	if c.writePadding < kFirstPaddings {
		bufferLen := buffer.Len()
		if bufferLen > 65535 {
			return common.Error(c.Write(buffer.Bytes()))
		}
		paddingSize := rand.Intn(256)
		header := buffer.ExtendHeader(3)
		binary.BigEndian.PutUint16(header, uint16(bufferLen))
		header[2] = byte(paddingSize)
		buffer.Extend(paddingSize)
		c.writePadding++
	}
	err := common.Error(c.writer.Write(buffer.Bytes()))
	if err == nil {
		c.flusher.Flush()
	}
	return wrapHttpError(err)
}

func (c *naiveH2Conn) WriteTo(w io.Writer) (n int64, err error) {
	if c.readPadding < kFirstPaddings {
		n, err = bufio.WriteToN(c, w, kFirstPaddings-c.readPadding)
	} else {
		n, err = bufio.Copy(w, c.reader)
	}
	return n, wrapHttpError(err)
}

func (c *naiveH2Conn) ReadFrom(r io.Reader) (n int64, err error) {
	if c.writePadding < kFirstPaddings {
		n, err = bufio.ReadFromN(c, r, kFirstPaddings-c.writePadding)
	} else {
		n, err = bufio.Copy(c.writer, r)
	}
	return n, wrapHttpError(err)
}

func (c *naiveH2Conn) Close() error {
	return common.Close(
		c.reader,
		c.writer,
	)
}

func (c *naiveH2Conn) LocalAddr() net.Addr {
	return nil
}

func (c *naiveH2Conn) RemoteAddr() net.Addr {
	return c.rAddr
}

func (c *naiveH2Conn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *naiveH2Conn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *naiveH2Conn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *naiveH2Conn) UpstreamReader() any {
	return c.reader
}

func (c *naiveH2Conn) UpstreamWriter() any {
	return c.writer
}

func (c *naiveH2Conn) ReaderReplaceable() bool {
	return c.readRemaining == kFirstPaddings
}

func (c *naiveH2Conn) WriterReplaceable() bool {
	return c.writePadding == kFirstPaddings
}

func wrapHttpError(err error) error {
	if err == nil {
		return err
	}
	if strings.Contains(err.Error(), "client disconnected") {
		return net.ErrClosed
	}
	if strings.Contains(err.Error(), "body closed by handler") {
		return net.ErrClosed
	}
	return err
}
