package inbound

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
)

var _ adapter.Inbound = (*Naive)(nil)

type Naive struct {
	ctx           context.Context
	router        adapter.Router
	logger        log.ContextLogger
	tag           string
	listenOptions option.ListenOptions
	network       []string
	authenticator auth.Authenticator
	tlsConfig     *TLSConfig
	httpServer    *http.Server
	h3Server      any
}

func NewNaive(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.NaiveInboundOptions) (*Naive, error) {
	inbound := &Naive{
		ctx:           ctx,
		router:        router,
		logger:        logger,
		tag:           tag,
		listenOptions: options.ListenOptions,
		network:       options.Network.Build(),
		authenticator: auth.NewAuthenticator(options.Users),
	}
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	if len(options.Users) == 0 {
		return nil, E.New("missing users")
	}
	tlsConfig, err := NewTLSConfig(ctx, logger, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	inbound.tlsConfig = tlsConfig
	return inbound, nil
}

func (n *Naive) Type() string {
	return C.TypeNaive
}

func (n *Naive) Tag() string {
	return n.tag
}

func (n *Naive) Start() error {
	err := n.tlsConfig.Start()
	if err != nil {
		return E.Cause(err, "create TLS config")
	}

	var listenAddr string
	if nAddr := netip.Addr(n.listenOptions.Listen); nAddr.IsValid() {
		if n.listenOptions.ListenPort != 0 {
			listenAddr = M.SocksaddrFrom(netip.Addr(n.listenOptions.Listen), n.listenOptions.ListenPort).String()
		} else {
			listenAddr = net.JoinHostPort(nAddr.String(), ":https")
		}
	} else if n.listenOptions.ListenPort != 0 {
		listenAddr = ":" + F.ToString(n.listenOptions.ListenPort)
	} else {
		listenAddr = ":https"
	}

	if common.Contains(n.network, N.NetworkTCP) {
		n.httpServer = &http.Server{
			Handler:   n,
			TLSConfig: n.tlsConfig.Config(),
		}
		tcpListener, err := net.Listen(M.NetworkFromNetAddr("tcp", netip.Addr(n.listenOptions.Listen)), listenAddr)
		if err != nil {
			return err
		}
		n.logger.Info("tcp server started at ", tcpListener.Addr())
		go func() {
			sErr := n.httpServer.ServeTLS(tcpListener, "", "")
			if sErr == http.ErrServerClosed {
			} else if sErr != nil {
				n.logger.Error("http server serve error: ", sErr)
			}
		}()
	}

	if common.Contains(n.network, N.NetworkUDP) {
		err = n.configureHTTP3Listener(listenAddr)
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
		common.PtrOrNil(n.httpServer),
		n.h3Server,
		common.PtrOrNil(n.tlsConfig),
	)
}

func (n *Naive) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx := log.ContextWithNewID(request.Context())
	if request.Method != "CONNECT" {
		n.logger.ErrorContext(ctx, "bad request: not connect")
		rejectHTTP(writer, http.StatusBadRequest)
		return
	} else if request.Header.Get("Padding") == "" {
		n.logger.ErrorContext(ctx, "bad request: missing padding")
		rejectHTTP(writer, http.StatusBadRequest)
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
		n.logger.ErrorContext(ctx, "bad request: authorization failed")
		rejectHTTP(writer, http.StatusProxyAuthRequired)
		return
	}
	writer.Header().Set("Padding", generateNaivePaddingHeader())
	writer.WriteHeader(http.StatusOK)
	writer.(http.Flusher).Flush()

	if request.ProtoMajor == 1 {
		n.logger.ErrorContext(ctx, "bad request: http1")
		rejectHTTP(writer, http.StatusBadRequest)
		return
	}

	hostPort := request.URL.Host
	if hostPort == "" {
		hostPort = request.Host
	}
	source := M.ParseSocksaddr(request.RemoteAddr)
	destination := M.ParseSocksaddr(hostPort)
	n.newConnection(ctx, &naivePaddingConn{reader: request.Body, writer: writer, flusher: writer.(http.Flusher)}, source, destination)
}

func (n *Naive) newConnection(ctx context.Context, conn net.Conn, source, destination M.Socksaddr) {
	var metadata adapter.InboundContext
	metadata.Inbound = n.tag
	metadata.InboundType = C.TypeNaive
	metadata.SniffEnabled = n.listenOptions.SniffEnabled
	metadata.SniffOverrideDestination = n.listenOptions.SniffOverrideDestination
	metadata.DomainStrategy = dns.DomainStrategy(n.listenOptions.DomainStrategy)
	metadata.Network = N.NetworkTCP
	metadata.Source = source
	metadata.Destination = destination
	n.logger.InfoContext(ctx, "inbound connection from ", metadata.Source)
	n.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	hErr := n.router.RouteConnection(ctx, conn, metadata)
	if hErr != nil {
		conn.Close()
		NewError(n.logger, ctx, E.Cause(hErr, "process connection from ", metadata.Source))
	}
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

var _ net.Conn = (*naivePaddingConn)(nil)

type naivePaddingConn struct {
	reader           io.Reader
	writer           io.Writer
	flusher          http.Flusher
	rAddr            net.Addr
	readPadding      int
	writePadding     int
	readRemaining    int
	paddingRemaining int
}

func (c *naivePaddingConn) Read(p []byte) (n int, err error) {
	n, err = c.read(p)
	return n, wrapHttpError(err)
}

func (c *naivePaddingConn) read(p []byte) (n int, err error) {
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

func (c *naivePaddingConn) Write(p []byte) (n int, err error) {
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

func (c *naivePaddingConn) write(p []byte) (n int, err error) {
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

func (c *naivePaddingConn) FrontHeadroom() int {
	if c.writePadding < kFirstPaddings {
		return 3
	}
	return 0
}

func (c *naivePaddingConn) RearHeadroom() int {
	if c.writePadding < kFirstPaddings {
		return 255
	}
	return 0
}

func (c *naivePaddingConn) WriterMTU() int {
	if c.writePadding < kFirstPaddings {
		return 65535
	}
	return 0
}

func (c *naivePaddingConn) WriteBuffer(buffer *buf.Buffer) error {
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

func (c *naivePaddingConn) WriteTo(w io.Writer) (n int64, err error) {
	if c.readPadding < kFirstPaddings {
		n, err = bufio.WriteToN(c, w, kFirstPaddings-c.readPadding)
	} else {
		n, err = bufio.Copy(w, c.reader)
	}
	return n, wrapHttpError(err)
}

func (c *naivePaddingConn) ReadFrom(r io.Reader) (n int64, err error) {
	if c.writePadding < kFirstPaddings {
		n, err = bufio.ReadFromN(c, r, kFirstPaddings-c.writePadding)
	} else {
		n, err = bufio.Copy(c.writer, r)
	}
	return n, wrapHttpError(err)
}

func (c *naivePaddingConn) Close() error {
	return common.Close(
		c.reader,
		c.writer,
	)
}

func (c *naivePaddingConn) LocalAddr() net.Addr {
	return nil
}

func (c *naivePaddingConn) RemoteAddr() net.Addr {
	return c.rAddr
}

func (c *naivePaddingConn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *naivePaddingConn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *naivePaddingConn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
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
