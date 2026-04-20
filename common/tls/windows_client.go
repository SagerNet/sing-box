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
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

const (
	windowsTLSEngineName   = "Windows TLS engine"
	handshakeReadChunkSize = 8192
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
	return &windowsTLSConn{
		rawConn:      conn,
		client:       client,
		state:        state,
		header:       header,
		trailer:      trailer,
		maxMessage:   maxMessage,
		cipher:       leftover,
		plainStorage: make([]byte, maxMessage),
		readScratch:  make([]byte, 16*1024),
		writeScratch: make([]byte, int(header)+int(maxMessage)+int(trailer)),
		closed:       make(chan struct{}),
	}, nil
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
	// Schannel reports how much input it consumed and whether it needs more.
	// Keep feeding peer bytes until the handshake completes, then return any
	// leftover ciphertext or coalesced application data to the caller.
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

	readAccess   sync.Mutex
	writeAccess  sync.Mutex
	clientAccess sync.Mutex
	closeOnce    sync.Once

	writeState     sync.Mutex
	writeStateOnce sync.Once
	writeReady     *sync.Cond
	postHandshake  bool
	writeActive    bool

	cipher       []byte
	plain        []byte
	plainStorage []byte
	readScratch  []byte
	writeScratch []byte
	readEOF      bool

	deadlineAccess sync.Mutex
	readDeadline   time.Time
	writeDeadline  time.Time
	closed         chan struct{}
}

func (c *windowsTLSConn) Read(p []byte) (int, error) {
	c.readAccess.Lock()
	defer c.readAccess.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	if c.isClosed() {
		return 0, net.ErrClosed
	}

	if len(c.plain) > 0 {
		n := copy(p, c.plain)
		c.plain = c.plain[n:]
		return n, nil
	}
	if c.readEOF {
		return 0, io.EOF
	}

	cleanup, err := c.applyReadDeadline()
	if err != nil {
		return 0, err
	}
	defer cleanup()

	for {
		if len(c.cipher) > 0 {
			c.clientAccess.Lock()
			if c.client == nil {
				c.clientAccess.Unlock()
				return 0, net.ErrClosed
			}
			result, decryptErr := c.client.Decrypt(c.cipher)
			if decryptErr != nil {
				c.clientAccess.Unlock()
				return 0, decryptErr
			}
			if result.Expired {
				c.clientAccess.Unlock()
				c.readEOF = true
				return 0, io.EOF
			}
			if !result.Incomplete {
				// Extract plaintext into caller buffer + overflow while the
				// cipher memory is still valid; Plaintext aliases c.cipher.
				n := copy(p, result.Plaintext)
				var extra []byte
				if n < len(result.Plaintext) {
					count := copy(c.plainStorage, result.Plaintext[n:])
					extra = c.plainStorage[:count]
				}
				c.cipher = c.cipher[result.ConsumedTotal:]
				if len(c.cipher) == 0 {
					c.cipher = nil
				}
				c.clientAccess.Unlock()
				if result.Renegotiate {
					postErr := c.drivePostHandshake()
					if postErr != nil {
						return 0, postErr
					}
				}
				c.plain = extra
				if n > 0 {
					return n, nil
				}
				continue
			}
			c.clientAccess.Unlock()
		}
		more, readErr := c.readRaw(len(c.cipher) > 0)
		if readErr != nil {
			return 0, readErr
		}
		c.cipher = append(c.cipher, more...)
	}
}

// c.cipher already contains the raw post-handshake record that Schannel wants
// fed back through InitializeSecurityContext.
func (c *windowsTLSConn) drivePostHandshake() error {
	initial := c.cipher
	c.cipher = nil
	err := c.beginPostHandshakeWrite()
	if err != nil {
		return err
	}
	defer c.finishPostHandshakeWrite()
	c.clientAccess.Lock()
	if c.client == nil {
		c.clientAccess.Unlock()
		return net.ErrClosed
	}
	writeFailed := false
	readMore := func() ([]byte, error) {
		more, err := c.readRaw(true)
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
	c.clientAccess.Unlock()
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

func (c *windowsTLSConn) writePostHandshakeReply(data []byte) error {
	c.writeAccess.Lock()
	defer c.writeAccess.Unlock()
	err := c.writePostHandshakeReplyLocked(data)
	if err != nil {
		_ = c.Close()
	}
	return err
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

func (c *windowsTLSConn) readRaw(requireMore bool) ([]byte, error) {
	return readTLSRaw(c.rawConn, c.readScratch, requireMore)
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
		c.clientAccess.Lock()
		if c.client == nil {
			c.clientAccess.Unlock()
			return total, net.ErrClosed
		}
		encrypted, encryptErr := c.client.Encrypt(c.header, c.trailer, chunk, c.writeScratch)
		c.clientAccess.Unlock()
		if encryptErr != nil {
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

func (c *windowsTLSConn) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		close(c.closed)
		ready := c.writeCondition()
		c.writeState.Lock()
		ready.Broadcast()
		c.writeState.Unlock()
		closeErr = c.rawConn.Close()
		c.clientAccess.Lock()
		if c.client != nil {
			c.client.Close()
			c.client = nil
		}
		c.clientAccess.Unlock()
	})
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
	c.readDeadline = t
	c.writeDeadline = t
	c.deadlineAccess.Unlock()
	return nil
}

func (c *windowsTLSConn) SetReadDeadline(t time.Time) error {
	c.deadlineAccess.Lock()
	c.readDeadline = t
	c.deadlineAccess.Unlock()
	return nil
}

func (c *windowsTLSConn) SetWriteDeadline(t time.Time) error {
	c.deadlineAccess.Lock()
	c.writeDeadline = t
	c.deadlineAccess.Unlock()
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
		if c.isClosed() {
			c.writeState.Unlock()
			return net.ErrClosed
		}
		ready.Wait()
	}
	c.writeActive = true
	c.writeState.Unlock()

	c.writeAccess.Lock()

	if c.isClosed() {
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
		if c.isClosed() {
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
	if c.isClosed() {
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
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}
