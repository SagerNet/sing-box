//go:build windows

package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/common/schannel"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
)

const (
	windowsTLSEngineName        = "Windows TLS engine"
	handshakeReadChunkSize      = 8192
	readScratchSize             = 16 * 1024
	readWaitCiphertextChunkSize = 4096
)

type windowsClientConfig struct {
	systemTLSConfig
	userRoots *x509.CertPool
}

func (c *windowsClientConfig) Clone() Config {
	return &windowsClientConfig{
		systemTLSConfig: c.systemTLSConfig.clone(),
		userRoots:       c.userRoots,
	}
}

func newWindowsClient(ctx context.Context, logger logger.ContextLogger, serverAddress string, options option.OutboundTLSOptions, allowEmptyServerName bool) (Config, error) {
	err := schannel.CheckPlatform()
	if err != nil {
		return nil, err
	}
	base, validated, err := newSystemTLSConfig(ctx, serverAddress, options, allowEmptyServerName, windowsTLSEngineName)
	if err != nil {
		return nil, err
	}
	var userRoots *x509.CertPool
	if len(validated.UserPEM) > 0 {
		userRoots = x509.NewCertPool()
		if !userRoots.AppendCertsFromPEM(validated.UserPEM) {
			return nil, E.New("parse certificate PEM")
		}
	}
	return &windowsClientConfig{
		systemTLSConfig: base,
		userRoots:       userRoots,
	}, nil
}

func (c *windowsClientConfig) ClientHandshake(ctx context.Context, conn net.Conn) (Conn, error) {
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		deadlineErr := conn.SetDeadline(deadline)
		if deadlineErr != nil {
			return nil, E.Cause(deadlineErr, "set handshake deadline")
		}
		defer conn.SetDeadline(time.Time{})
	}

	client, err := schannel.NewClientContext(c.minVersion, c.maxVersion, c.serverName, c.nextProtos)
	if err != nil {
		return nil, err
	}

	handshakeOK := false
	defer func() {
		if !handshakeOK {
			client.Close()
		}
	}()

	stopCancel := installHandshakeCancel(ctx, conn)
	defer stopCancel()

	scratch := make([]byte, handshakeReadChunkSize)
	leftover, err := driveHandshake(ctx, conn, client, scratch)
	if err != nil {
		return nil, err
	}
	state, rawCerts, err := buildConnectionState(c.serverName, client)
	if err != nil {
		return nil, err
	}
	err = c.verifyPeerCertificates(state.PeerCertificates)
	if err != nil {
		return nil, err
	}
	if len(c.certificatePublicKeySHA256) > 0 {
		err = VerifyPublicKeySHA256(c.certificatePublicKeySHA256, rawCerts)
		if err != nil {
			return nil, err
		}
	}
	header, trailer, maxMessage, err := client.StreamSizes()
	if err != nil {
		return nil, err
	}

	handshakeOK = true
	tlsConn := &windowsTLSConn{
		rawConn:    conn,
		client:     client,
		state:      state,
		header:     header,
		trailer:    trailer,
		maxMessage: maxMessage,
		cipher:     leftover,
	}
	return tlsConn, nil
}

func driveHandshake(ctx context.Context, conn net.Conn, client *schannel.ClientContext, scratch []byte) ([]byte, error) {
	readMore := func() ([]byte, error) {
		more, err := readTLSRaw(conn, scratch, true)
		if err != nil {
			return nil, handshakeIOError(ctx, err, "read handshake")
		}
		return more, nil
	}
	writeOut := func(data []byte) error {
		_, err := conn.Write(data)
		if err != nil {
			return handshakeIOError(ctx, err, "write handshake")
		}
		return nil
	}
	leftover, err := driveSteps(nil, client.Step, readMore, writeOut)
	if err != nil {
		return nil, E.Cause(err, "tls handshake")
	}
	return leftover, nil
}

func driveSteps(
	initial []byte,
	step func([]byte) (schannel.StepResult, error),
	readMore func() ([]byte, error),
	writeOut func([]byte) error,
) ([]byte, error) {
	buffer := initial
	for {
		result, stepErr := step(buffer)
		if stepErr != nil {
			return nil, stepErr
		}
		if len(result.Output) > 0 {
			writeErr := writeOut(result.Output)
			if writeErr != nil {
				return nil, writeErr
			}
		}
		if result.Incomplete {
			// readMore reuses scratch storage, so keep the buffered handshake
			// bytes in stable memory before the next read overwrites them.
			buffer = append([]byte(nil), buffer...)
			more, readErr := readMore()
			if readErr != nil {
				return nil, readErr
			}
			buffer = append(buffer, more...)
			continue
		}
		if result.Consumed > len(buffer) {
			return nil, E.New("schannel: Consumed > input length")
		}
		buffer = buffer[result.Consumed:]
		if result.Done {
			return buffer, nil
		}
		if len(buffer) == 0 {
			more, readErr := readMore()
			if readErr != nil {
				return nil, readErr
			}
			buffer = append(buffer, more...)
		}
	}
}

// installHandshakeCancel unblocks an in-flight read/write by forcing an
// immediate deadline on conn when ctx is cancelled. The returned cleanup
// waits for a racing cancel to finish and clears the forced deadline.
func installHandshakeCancel(ctx context.Context, conn net.Conn) func() {
	var fired atomic.Bool
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(done)
		fired.Store(true)
		_ = conn.SetDeadline(time.Now())
	})
	return func() {
		if stop() {
			return
		}
		<-done
		if fired.Load() {
			_ = conn.SetDeadline(time.Time{})
		}
	}
}

func handshakeIOError(ctx context.Context, err error, message string) error {
	ctxErr := ctx.Err()
	if ctxErr != nil && isTimeoutError(err) {
		return ctxErr
	}
	return E.Cause(err, message)
}

func readTLSRaw(conn net.Conn, scratch []byte, requireMore bool) ([]byte, error) {
	n, err := conn.Read(scratch)
	if n > 0 {
		return scratch[:n], nil
	}
	if err != nil {
		if requireMore && errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}
	return nil, io.ErrUnexpectedEOF
}

func isTimeoutError(err error) bool {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func buildConnectionState(serverName string, client *schannel.ClientContext) (tls.ConnectionState, [][]byte, error) {
	version, cipherSuite, err := client.ConnectionInfo()
	if err != nil {
		return tls.ConnectionState{}, nil, err
	}
	alpn, err := client.ApplicationProtocol()
	if err != nil {
		return tls.ConnectionState{}, nil, err
	}
	rawCerts, err := client.RemoteCertificateChain()
	if err != nil {
		return tls.ConnectionState{}, nil, err
	}
	peerCertificates := make([]*x509.Certificate, 0, len(rawCerts))
	for index, der := range rawCerts {
		cert, parseErr := x509.ParseCertificate(der)
		if parseErr != nil {
			return tls.ConnectionState{}, nil, E.Cause(parseErr, "parse peer certificate ", index)
		}
		peerCertificates = append(peerCertificates, cert)
	}
	return tls.ConnectionState{
		Version:            version,
		HandshakeComplete:  true,
		CipherSuite:        cipherSuite,
		NegotiatedProtocol: alpn,
		ServerName:         serverName,
		PeerCertificates:   peerCertificates,
	}, rawCerts, nil
}

func (c *windowsClientConfig) verifyPeerCertificates(peerCertificates []*x509.Certificate) error {
	if c.insecure {
		return nil
	}
	var roots *x509.CertPool
	switch {
	case c.userRoots != nil:
		roots = c.userRoots
	case c.store != nil:
		roots = c.store.Pool()
	}
	return verifySystemTLSPeer(roots, c.serverName, c.timeFunc, peerCertificates)
}

type windowsTLSConn struct {
	rawConn    net.Conn
	client     *schannel.ClientContext
	state      tls.ConnectionState
	header     uint32
	trailer    uint32
	maxMessage uint32

	readAccess    sync.Mutex
	writeAccess   sync.Mutex
	contextAccess sync.RWMutex

	writeState     sync.Mutex
	writeStateOnce sync.Once
	writeReady     *sync.Cond
	postHandshake  bool
	writeActive    bool

	cipher       []byte
	plain        []byte
	readScratch  []byte
	writeScratch []byte
	readEOF      bool

	deadlineAccess sync.Mutex
	readDeadline   time.Time
	writeDeadline  time.Time
	closed         atomic.Bool
}

var (
	_ N.ExtendedConn    = (*windowsTLSConn)(nil)
	_ N.ReadWaitCreator = (*windowsTLSConn)(nil)
)

type (
	windowsTLSAppendCipherFunc func(requireMore bool) error
	windowsTLSReadRawFunc      func(requireMore bool) ([]byte, error)
)

func (c *windowsTLSConn) Read(p []byte) (int, error) {
	c.readAccess.Lock()
	defer c.readAccess.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	if c.isClosed() {
		return 0, net.ErrClosed
	}
	return c.readIntoLocked(p, c.appendRaw, c.readRaw)
}

func (c *windowsTLSConn) ReadBuffer(buffer *buf.Buffer) error {
	c.readAccess.Lock()
	defer c.readAccess.Unlock()
	if buffer.IsFull() {
		return io.ErrShortBuffer
	}
	if c.isClosed() {
		return net.ErrClosed
	}
	startLen := buffer.Len()
	n, err := c.readIntoLocked(buffer.FreeBytes(), c.appendRaw, c.readRaw)
	buffer.Truncate(startLen + n)
	return err
}

func (c *windowsTLSConn) readIntoLocked(p []byte, appendCipher windowsTLSAppendCipherFunc, readRaw windowsTLSReadRawFunc) (int, error) {
	plaintext, err := c.readPlaintextLocked(appendCipher, readRaw)
	if err != nil {
		return 0, err
	}
	n := copy(p, plaintext)
	if n < len(plaintext) {
		c.plain = append([]byte(nil), plaintext[n:]...)
	}
	return n, nil
}

func (c *windowsTLSConn) readPlaintextLocked(appendCipher windowsTLSAppendCipherFunc, readRaw windowsTLSReadRawFunc) ([]byte, error) {
	if len(c.plain) > 0 {
		plaintext := c.plain
		c.plain = nil
		return plaintext, nil
	}
	if c.readEOF {
		return nil, io.EOF
	}

	cleanup, err := c.applyReadDeadline()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	for {
		if len(c.cipher) > 0 {
			result, decryptErr := c.decrypt(c.cipher)
			if decryptErr != nil {
				return nil, decryptErr
			}
			if result.Expired {
				c.readEOF = true
				return nil, io.EOF
			}
			if !result.Incomplete {
				plaintext := result.Plaintext
				if result.Renegotiate && len(plaintext) > 0 {
					plaintext = append([]byte(nil), plaintext...)
				}
				nextCipher := c.cipher[result.ConsumedTotal:]
				if len(result.RenegotiateToken) > 0 {
					nextCipher = result.RenegotiateToken
				}
				c.cipher = nextCipher
				if len(c.cipher) == 0 {
					c.cipher = nil
				}
				if result.Renegotiate {
					postErr := c.drivePostHandshake(readRaw)
					if postErr != nil {
						return nil, postErr
					}
				}
				if len(plaintext) > 0 {
					return plaintext, nil
				}
				continue
			}
		}
		err = appendCipher(len(c.cipher) > 0)
		if err != nil {
			return nil, err
		}
	}
}

func (c *windowsTLSConn) drivePostHandshake(readRaw windowsTLSReadRawFunc) error {
	initial := c.cipher
	c.cipher = nil
	err := c.beginPostHandshakeWrite()
	if err != nil {
		return err
	}
	defer c.finishPostHandshakeWrite()
	c.contextAccess.Lock()
	if c.client == nil {
		c.contextAccess.Unlock()
		return net.ErrClosed
	}
	writeFailed := false
	readMore := func() ([]byte, error) {
		more, err := readRaw(true)
		if err != nil {
			return nil, E.Cause(err, "tls post-handshake read")
		}
		return more, nil
	}
	writeOut := func(data []byte) error {
		err := c.writePostHandshakeReplyLocked(data)
		if err != nil {
			writeFailed = true
			return E.Cause(err, "tls post-handshake write")
		}
		return nil
	}
	leftover, err := driveSteps(initial, c.client.PostHandshake, readMore, writeOut)
	c.contextAccess.Unlock()
	if err != nil {
		if writeFailed {
			_ = c.Close()
		}
		return E.Cause(err, "tls post-handshake")
	}
	if len(leftover) > 0 {
		c.cipher = leftover
	}
	return nil
}

func (c *windowsTLSConn) writePostHandshakeReplyLocked(data []byte) error {
	c.deadlineAccess.Lock()
	deadline := c.readDeadline
	c.deadlineAccess.Unlock()
	cleanup, err := c.applyDeadline(deadline, c.rawConn.SetWriteDeadline)
	if err != nil {
		return err
	}
	defer cleanup()
	_, err = c.rawConn.Write(data)
	return err
}

func (c *windowsTLSConn) decrypt(input []byte) (schannel.DecryptResult, error) {
	c.contextAccess.RLock()
	defer c.contextAccess.RUnlock()
	if c.client == nil {
		return schannel.DecryptResult{}, net.ErrClosed
	}
	return c.client.Decrypt(input)
}

func (c *windowsTLSConn) encrypt(plaintext []byte) ([]byte, error) {
	c.contextAccess.RLock()
	defer c.contextAccess.RUnlock()
	if c.client == nil {
		return nil, net.ErrClosed
	}
	if c.writeScratch == nil {
		c.writeScratch = make([]byte, int(c.header)+int(c.maxMessage)+int(c.trailer))
	}
	return c.client.Encrypt(c.header, c.trailer, plaintext, c.writeScratch)
}

func (c *windowsTLSConn) readRaw(requireMore bool) ([]byte, error) {
	if c.readScratch == nil {
		c.readScratch = make([]byte, readScratchSize)
	}
	return readTLSRaw(c.rawConn, c.readScratch, requireMore)
}

func (c *windowsTLSConn) appendRaw(requireMore bool) error {
	more, err := c.readRaw(requireMore)
	if err != nil {
		return err
	}
	c.cipher = append(c.cipher, more...)
	return nil
}

func (c *windowsTLSConn) Write(p []byte) (int, error) {
	err := c.beginWrite()
	if err != nil {
		return 0, err
	}
	defer c.finishWrite()
	if len(p) == 0 {
		return 0, nil
	}
	if c.isClosed() {
		return 0, net.ErrClosed
	}

	cleanup, err := c.applyWriteDeadline()
	if err != nil {
		return 0, err
	}
	defer cleanup()

	total := 0
	chunkSize := int(c.maxMessage)
	for len(p) > 0 {
		chunk := p
		if len(chunk) > chunkSize {
			chunk = chunk[:chunkSize]
		}
		encrypted, encryptErr := c.encrypt(chunk)
		if encryptErr != nil {
			if errors.Is(encryptErr, net.ErrClosed) {
				return total, net.ErrClosed
			}
			return total, E.Cause(encryptErr, "tls encrypt")
		}
		_, writeErr := c.rawConn.Write(encrypted)
		if writeErr != nil {
			_ = c.Close()
			return total, E.Cause(writeErr, "tls write")
		}
		total += len(chunk)
		p = p[len(chunk):]
	}
	return total, nil
}

func (c *windowsTLSConn) WriteBuffer(buffer *buf.Buffer) error {
	defer buffer.Release()
	_, err := c.Write(buffer.Bytes())
	return err
}

func (c *windowsTLSConn) CreateReadWaiter() (N.ReadWaiter, bool) {
	rawWaiter, ok := bufio.CreateReadWaiter(c.rawConn)
	if !ok {
		return nil, false
	}
	return &windowsTLSReadWaiter{
		conn:      c,
		rawWaiter: rawWaiter,
	}, true
}

func (c *windowsTLSConn) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	ready := c.writeCondition()
	c.writeState.Lock()
	ready.Broadcast()
	c.writeState.Unlock()
	closeErr := c.rawConn.Close()
	c.contextAccess.Lock()
	if c.client != nil {
		c.client.Close()
		c.client = nil
	}
	c.contextAccess.Unlock()
	return closeErr
}

func (c *windowsTLSConn) LocalAddr() net.Addr {
	return c.rawConn.LocalAddr()
}

func (c *windowsTLSConn) RemoteAddr() net.Addr {
	return c.rawConn.RemoteAddr()
}

func (c *windowsTLSConn) SetDeadline(t time.Time) error {
	c.deadlineAccess.Lock()
	defer c.deadlineAccess.Unlock()
	err := c.rawConn.SetDeadline(t)
	if err != nil {
		return err
	}
	c.readDeadline = t
	c.writeDeadline = t
	return nil
}

func (c *windowsTLSConn) SetReadDeadline(t time.Time) error {
	c.deadlineAccess.Lock()
	defer c.deadlineAccess.Unlock()
	err := c.rawConn.SetReadDeadline(t)
	if err != nil {
		return err
	}
	c.readDeadline = t
	return nil
}

func (c *windowsTLSConn) SetWriteDeadline(t time.Time) error {
	c.deadlineAccess.Lock()
	defer c.deadlineAccess.Unlock()
	err := c.rawConn.SetWriteDeadline(t)
	if err != nil {
		return err
	}
	c.writeDeadline = t
	return nil
}

func (c *windowsTLSConn) NetConn() net.Conn {
	return c.rawConn
}

func (c *windowsTLSConn) HandshakeContext(ctx context.Context) error {
	return nil
}

func (c *windowsTLSConn) ConnectionState() ConnectionState {
	return c.state
}

func (c *windowsTLSConn) applyReadDeadline() (func(), error) {
	c.deadlineAccess.Lock()
	deadline := c.readDeadline
	c.deadlineAccess.Unlock()
	return c.applyDeadline(deadline, c.rawConn.SetReadDeadline)
}

func (c *windowsTLSConn) applyWriteDeadline() (func(), error) {
	c.deadlineAccess.Lock()
	deadline := c.writeDeadline
	c.deadlineAccess.Unlock()
	return c.applyDeadline(deadline, c.rawConn.SetWriteDeadline)
}

func (c *windowsTLSConn) applyDeadline(deadline time.Time, set func(time.Time) error) (func(), error) {
	if deadline.IsZero() {
		return func() {}, nil
	}
	if !deadline.After(time.Now()) {
		return nil, os.ErrDeadlineExceeded
	}
	err := set(deadline)
	if err != nil {
		return nil, err
	}
	return func() { _ = set(time.Time{}) }, nil
}

func (c *windowsTLSConn) beginWrite() error {
	ready := c.writeCondition()
	c.writeState.Lock()
	for c.postHandshake || c.writeActive {
		if c.closed.Load() {
			c.writeState.Unlock()
			return net.ErrClosed
		}
		ready.Wait()
	}
	c.writeActive = true
	c.writeState.Unlock()

	c.writeAccess.Lock()
	if c.closed.Load() {
		c.writeAccess.Unlock()
		c.writeState.Lock()
		c.writeActive = false
		ready.Broadcast()
		c.writeState.Unlock()
		return net.ErrClosed
	}
	return nil
}

func (c *windowsTLSConn) finishWrite() {
	c.writeAccess.Unlock()
	ready := c.writeCondition()
	c.writeState.Lock()
	c.writeActive = false
	ready.Broadcast()
	c.writeState.Unlock()
}

func (c *windowsTLSConn) beginPostHandshakeWrite() error {
	ready := c.writeCondition()
	c.writeState.Lock()
	c.postHandshake = true
	for c.writeActive {
		if c.closed.Load() {
			c.postHandshake = false
			ready.Broadcast()
			c.writeState.Unlock()
			return net.ErrClosed
		}
		ready.Wait()
	}
	c.writeActive = true
	c.writeState.Unlock()

	c.writeAccess.Lock()
	if c.closed.Load() {
		c.writeAccess.Unlock()
		c.writeState.Lock()
		c.writeActive = false
		c.postHandshake = false
		ready.Broadcast()
		c.writeState.Unlock()
		return net.ErrClosed
	}
	return nil
}

func (c *windowsTLSConn) finishPostHandshakeWrite() {
	c.writeAccess.Unlock()
	ready := c.writeCondition()
	c.writeState.Lock()
	c.writeActive = false
	c.postHandshake = false
	ready.Broadcast()
	c.writeState.Unlock()
}

func (c *windowsTLSConn) writeCondition() *sync.Cond {
	c.writeStateOnce.Do(func() {
		c.writeReady = sync.NewCond(&c.writeState)
	})
	return c.writeReady
}

func (c *windowsTLSConn) isClosed() bool {
	return c.closed.Load()
}

type windowsTLSReadWaiter struct {
	conn      *windowsTLSConn
	rawWaiter N.ReadWaiter
	options   N.ReadWaitOptions
}

var _ N.ReadWaiter = (*windowsTLSReadWaiter)(nil)

func (w *windowsTLSReadWaiter) InitializeReadWaiter(options N.ReadWaitOptions) (needCopy bool) {
	w.options = options
	w.rawWaiter.InitializeReadWaiter(N.ReadWaitOptions{
		MTU: readWaitCiphertextChunkSize,
	})
	return false
}

func (w *windowsTLSReadWaiter) WaitReadBuffer() (*buf.Buffer, error) {
	c := w.conn
	c.readAccess.Lock()
	defer c.readAccess.Unlock()
	if c.isClosed() {
		return nil, net.ErrClosed
	}
	plaintext, err := c.readPlaintextLocked(w.appendRaw, w.readRaw)
	if err != nil {
		return nil, err
	}
	buffer := w.options.NewBuffer()
	n, writeErr := buffer.Write(plaintext)
	if writeErr != nil {
		buffer.Release()
		return nil, writeErr
	}
	if n == 0 {
		buffer.Release()
		return nil, io.ErrShortBuffer
	}
	if n < len(plaintext) {
		c.plain = append([]byte(nil), plaintext[n:]...)
	}
	w.options.PostReturn(buffer)
	return buffer, nil
}

func (w *windowsTLSReadWaiter) appendRaw(requireMore bool) error {
	rawBuffer, err := w.readRawBuffer(requireMore)
	if err != nil {
		return err
	}
	w.conn.cipher = append(w.conn.cipher, rawBuffer.Bytes()...)
	rawBuffer.Release()
	return nil
}

func (w *windowsTLSReadWaiter) readRaw(requireMore bool) ([]byte, error) {
	rawBuffer, err := w.readRawBuffer(requireMore)
	if err != nil {
		return nil, err
	}
	data := append([]byte(nil), rawBuffer.Bytes()...)
	rawBuffer.Release()
	return data, nil
}

func (w *windowsTLSReadWaiter) readRawBuffer(requireMore bool) (*buf.Buffer, error) {
	rawBuffer, err := w.rawWaiter.WaitReadBuffer()
	if err != nil {
		if requireMore && errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}
	if rawBuffer == nil || rawBuffer.Len() == 0 {
		if rawBuffer != nil {
			rawBuffer.Release()
		}
		return nil, io.ErrUnexpectedEOF
	}
	return rawBuffer, nil
}
