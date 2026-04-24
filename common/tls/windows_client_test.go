//go:build windows

package tls

import (
	"bytes"
	"context"
	"crypto/sha256"
	stdtls "crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/common/schannel"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
)

const windowsTLSTestTimeout = 5 * time.Second

var (
	_ N.ExtendedConn    = (*windowsTLSConn)(nil)
	_ N.ReadWaitCreator = (*windowsTLSConn)(nil)
)

func newTestWindowsTLSConn(rawConn net.Conn) *windowsTLSConn {
	return &windowsTLSConn{rawConn: rawConn}
}

// writePostHandshakeReply wraps writePostHandshakeReplyLocked with the
// writeAccess locking and auto-close behavior that drivePostHandshake
// composes from beginPostHandshakeWrite/finishPostHandshakeWrite plus the
// writeFailed → Close branch.
// Kept here as a test seam.
func (c *windowsTLSConn) writePostHandshakeReply(data []byte) error {
	c.writeAccess.Lock()
	defer c.writeAccess.Unlock()
	err := c.writePostHandshakeReplyLocked(data)
	if err != nil {
		_ = c.Close()
	}
	return err
}

type windowsTLSServerResult struct {
	state stdtls.ConnectionState
	err   error
}

type windowsTestDeadlineConn struct {
	access          sync.Mutex
	readCalled      chan struct{}
	writeCalled     chan struct{}
	readCalledOnce  sync.Once
	writeCalledOnce sync.Once
	readDeadline    time.Time
	writeDeadline   time.Time
	readDeadlines   []time.Time
	writeDeadlines  []time.Time
}

type windowsTestWriteGateConn struct {
	writeCalled  chan struct{}
	releaseWrite chan struct{}
}

type windowsTestIOConn struct {
	access     sync.Mutex
	readErr    error
	writeErr   error
	writeN     int
	writeCalls int
	closed     bool
}

func (c *windowsTestDeadlineConn) Read(_ []byte) (int, error) {
	if c.readCalled != nil {
		c.readCalledOnce.Do(func() {
			close(c.readCalled)
		})
	}
	for {
		c.access.Lock()
		deadline := c.readDeadline
		c.access.Unlock()
		if deadline.IsZero() {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if !deadline.After(time.Now()) {
			return 0, os.ErrDeadlineExceeded
		}
		time.Sleep(time.Until(deadline))
		return 0, os.ErrDeadlineExceeded
	}
}

func (c *windowsTestDeadlineConn) Write(_ []byte) (int, error) {
	if c.writeCalled != nil {
		c.writeCalledOnce.Do(func() {
			close(c.writeCalled)
		})
	}
	for {
		c.access.Lock()
		deadline := c.writeDeadline
		c.access.Unlock()
		if deadline.IsZero() {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if !deadline.After(time.Now()) {
			return 0, os.ErrDeadlineExceeded
		}
		time.Sleep(time.Until(deadline))
		return 0, os.ErrDeadlineExceeded
	}
}

func (c *windowsTestDeadlineConn) Close() error {
	return nil
}

func (c *windowsTestDeadlineConn) LocalAddr() net.Addr {
	return windowsTestAddr("local")
}

func (c *windowsTestDeadlineConn) RemoteAddr() net.Addr {
	return windowsTestAddr("remote")
}

func (c *windowsTestDeadlineConn) SetDeadline(t time.Time) error {
	c.access.Lock()
	c.readDeadline = t
	c.writeDeadline = t
	c.readDeadlines = append(c.readDeadlines, t)
	c.writeDeadlines = append(c.writeDeadlines, t)
	c.access.Unlock()
	return nil
}

func (c *windowsTestDeadlineConn) SetReadDeadline(t time.Time) error {
	c.access.Lock()
	c.readDeadline = t
	c.readDeadlines = append(c.readDeadlines, t)
	c.access.Unlock()
	return nil
}

func (c *windowsTestDeadlineConn) SetWriteDeadline(t time.Time) error {
	c.access.Lock()
	c.writeDeadline = t
	c.writeDeadlines = append(c.writeDeadlines, t)
	c.access.Unlock()
	return nil
}

func (c *windowsTestDeadlineConn) recordedWriteDeadlines() []time.Time {
	c.access.Lock()
	defer c.access.Unlock()
	return append([]time.Time(nil), c.writeDeadlines...)
}

func (c *windowsTestDeadlineConn) recordedReadDeadlines() []time.Time {
	c.access.Lock()
	defer c.access.Unlock()
	return append([]time.Time(nil), c.readDeadlines...)
}

func (c *windowsTestWriteGateConn) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (c *windowsTestWriteGateConn) Write(p []byte) (int, error) {
	close(c.writeCalled)
	<-c.releaseWrite
	return len(p), nil
}

func (c *windowsTestWriteGateConn) Close() error {
	return nil
}

func (c *windowsTestWriteGateConn) LocalAddr() net.Addr {
	return windowsTestAddr("local")
}

func (c *windowsTestWriteGateConn) RemoteAddr() net.Addr {
	return windowsTestAddr("remote")
}

func (c *windowsTestWriteGateConn) SetDeadline(time.Time) error {
	return nil
}

func (c *windowsTestWriteGateConn) SetReadDeadline(time.Time) error {
	return nil
}

func (c *windowsTestWriteGateConn) SetWriteDeadline(time.Time) error {
	return nil
}

func (c *windowsTestIOConn) Read(_ []byte) (int, error) {
	return 0, c.readErr
}

func (c *windowsTestIOConn) Write(p []byte) (int, error) {
	c.access.Lock()
	defer c.access.Unlock()
	c.writeCalls++
	if c.writeErr == nil {
		return len(p), nil
	}
	n := c.writeN
	if n <= 0 || n > len(p) {
		n = 0
	}
	return n, c.writeErr
}

func (c *windowsTestIOConn) Close() error {
	c.access.Lock()
	c.closed = true
	c.access.Unlock()
	return nil
}

func (c *windowsTestIOConn) LocalAddr() net.Addr {
	return windowsTestAddr("local")
}

func (c *windowsTestIOConn) RemoteAddr() net.Addr {
	return windowsTestAddr("remote")
}

func (c *windowsTestIOConn) SetDeadline(time.Time) error {
	return nil
}

func (c *windowsTestIOConn) SetReadDeadline(time.Time) error {
	return nil
}

func (c *windowsTestIOConn) SetWriteDeadline(time.Time) error {
	return nil
}

func (c *windowsTestIOConn) isClosed() bool {
	c.access.Lock()
	defer c.access.Unlock()
	return c.closed
}

func (c *windowsTestIOConn) totalWriteCalls() int {
	c.access.Lock()
	defer c.access.Unlock()
	return c.writeCalls
}

type windowsTestAddr string

func (a windowsTestAddr) Network() string {
	return "test"
}

func (a windowsTestAddr) String() string {
	return string(a)
}

type windowsOpaqueConn struct {
	net.Conn
}

func TestWindowsClientHandshakeTLS12(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	serverResult, serverAddress := startWindowsTLSTestServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS12,
		MaxVersion:   stdtls.VersionTLS12,
		NextProtos:   []string{"h2"},
	})

	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.2",
		MaxVersion:  "1.2",
		ALPN:        badoption.Listable[string]{"h2"},
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	clientState := clientConn.ConnectionState()
	if clientState.Version != stdtls.VersionTLS12 {
		t.Fatalf("unexpected negotiated version: %x", clientState.Version)
	}
	if clientState.NegotiatedProtocol != "h2" {
		t.Fatalf("unexpected negotiated protocol: %q", clientState.NegotiatedProtocol)
	}
	if !clientState.HandshakeComplete {
		t.Fatal("HandshakeComplete is false")
	}
	if len(clientState.PeerCertificates) == 0 {
		t.Fatal("no peer certificates")
	}

	result := <-serverResult
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.state.Version != stdtls.VersionTLS12 {
		t.Fatalf("server negotiated unexpected version: %x", result.state.Version)
	}
	if result.state.NegotiatedProtocol != "h2" {
		t.Fatalf("server negotiated unexpected protocol: %q", result.state.NegotiatedProtocol)
	}
}

func TestWindowsClientHandshakeWrappedConn(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	serverResult, serverAddress := startWindowsTLSTestServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS12,
		MaxVersion:   stdtls.VersionTLS12,
	})

	ctx, cancel := context.WithTimeout(context.Background(), windowsTLSTestTimeout)
	t.Cleanup(cancel)

	clientConfig, err := NewClientWithOptions(ClientOptions{
		Context: ctx,
		Logger:  logger.NOP(),
		Options: option.OutboundTLSOptions{
			Enabled:     true,
			Engine:      C.TLSEngineWindows,
			ServerName:  "localhost",
			MinVersion:  "1.2",
			Certificate: badoption.Listable[string]{serverCertificatePEM},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rawConn, err := net.DialTimeout(N.NetworkTCP, serverAddress, windowsTLSTestTimeout)
	if err != nil {
		t.Fatal(err)
	}
	tlsConn, err := ClientHandshake(ctx, windowsOpaqueConn{Conn: rawConn}, clientConfig)
	if err != nil {
		rawConn.Close()
		t.Fatal(err)
	}
	_ = tlsConn.Close()

	result := <-serverResult
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.state.Version != stdtls.VersionTLS12 {
		t.Fatalf("server negotiated unexpected version: %x", result.state.Version)
	}
}

func TestWindowsClientHandshakeTLS13(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	serverResult, serverAddress := startWindowsTLSTestServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS13,
		MaxVersion:   stdtls.VersionTLS13,
		NextProtos:   []string{"h2"},
	})

	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.3",
		ALPN:        badoption.Listable[string]{"h2"},
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	clientState := clientConn.ConnectionState()
	if clientState.Version != stdtls.VersionTLS13 {
		t.Fatalf("expected TLS 1.3, got %x", clientState.Version)
	}
	if clientState.NegotiatedProtocol != "h2" {
		t.Fatalf("expected negotiated protocol h2, got %q", clientState.NegotiatedProtocol)
	}

	result := <-serverResult
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.state.Version != stdtls.VersionTLS13 {
		t.Fatalf("server negotiated unexpected version: %x", result.state.Version)
	}
}

func TestWindowsClientHandshakeALPNNoOverlap(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	_, serverAddress := startWindowsTLSTestServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS12,
		MaxVersion:   stdtls.VersionTLS12,
		NextProtos:   []string{"http/1.1"},
	})

	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.2",
		MaxVersion:  "1.2",
		ALPN:        badoption.Listable[string]{"h2"},
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	// Go's TLS server returns a TLS alert when the client advertises ALPN but
	// the server has no overlap. The handshake fails.
	if err == nil {
		_ = clientConn.Close()
		t.Fatal("expected handshake to fail with no ALPN overlap")
	}
}

func TestWindowsClientHandshakeMultipleALPN(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	_, serverAddress := startWindowsTLSTestServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS12,
		NextProtos:   []string{"h2", "http/1.1"},
	})

	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.2",
		ALPN:        badoption.Listable[string]{"spdy/3", "h2"},
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	// Schannel follows the standard selection: first protocol offered by
	// the client that the server also supports. Here: spdy/3 is not in the
	// server list but h2 is, so h2 wins.
	if got := clientConn.ConnectionState().NegotiatedProtocol; got != "h2" {
		t.Fatalf("expected h2, got %q", got)
	}
}

func TestWindowsClientHandshakeRejectsVersionMismatch(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	serverResult, serverAddress := startWindowsTLSTestServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS13,
		MaxVersion:   stdtls.VersionTLS13,
	})

	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MaxVersion:  "1.2",
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err == nil {
		clientConn.Close()
		t.Fatal("expected version mismatch handshake to fail")
	}

	result := <-serverResult
	if result.err == nil {
		t.Fatal("expected server handshake to fail on version mismatch")
	}
}

func TestWindowsClientHandshakeRejectsServerNameMismatch(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	_, serverAddress := startWindowsTLSTestServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
	})

	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "example.com",
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err == nil {
		clientConn.Close()
		t.Fatal("expected server name mismatch handshake to fail")
	}
}

func TestWindowsClientHandshakeRejectsUntrustedCA(t *testing.T) {
	serverCertificate, _ := newWindowsTestCertificate(t, "localhost")
	_, serverAddress := startWindowsTLSTestServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
	})

	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:    true,
		Engine:     C.TLSEngineWindows,
		ServerName: "localhost",
	})
	if err == nil {
		clientConn.Close()
		t.Fatal("expected untrusted CA handshake to fail")
	}
}

func TestWindowsClientHandshakeInsecureSkipsValidation(t *testing.T) {
	serverCertificate, _ := newWindowsTestCertificate(t, "localhost")
	_, serverAddress := startWindowsTLSTestServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
	})

	// Server name mismatch but insecure=true → handshake succeeds.
	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:    true,
		Engine:     C.TLSEngineWindows,
		ServerName: "example.com",
		Insecure:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	if !clientConn.ConnectionState().HandshakeComplete {
		t.Fatal("expected handshake to complete with insecure=true")
	}
}

func TestWindowsClientHandshakeHonorsPublicKeyPinSuccess(t *testing.T) {
	serverCertificate, _ := newWindowsTestCertificate(t, "localhost")
	pin := publicKeyPin(t, serverCertificate.Leaf)
	_, serverAddress := startWindowsTLSTestServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
	})

	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:                    true,
		Engine:                     C.TLSEngineWindows,
		ServerName:                 "localhost",
		CertificatePublicKeySHA256: [][]byte{pin},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()
}

func TestWindowsClientHandshakeHonorsPublicKeyPinFailure(t *testing.T) {
	serverCertificate, _ := newWindowsTestCertificate(t, "localhost")
	wrongPin := sha256.Sum256([]byte("not the public key"))
	_, serverAddress := startWindowsTLSTestServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
	})

	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:                    true,
		Engine:                     C.TLSEngineWindows,
		ServerName:                 "localhost",
		CertificatePublicKeySHA256: [][]byte{wrongPin[:]},
	})
	if err == nil {
		clientConn.Close()
		t.Fatal("expected public-key pin mismatch to fail")
	}
}

func TestWindowsClientHandshakeContextCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	clientHelloRead := make(chan struct{}, 1)
	serverDone := make(chan struct{})
	defer close(serverDone)
	go func() {
		c, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer c.Close()
		buffer := make([]byte, 8192)
		n, readErr := c.Read(buffer)
		if n > 0 {
			clientHelloRead <- struct{}{}
		}
		if readErr != nil {
			return
		}
		<-serverDone
	}()

	_, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")

	ctx, cancel := context.WithCancel(context.Background())
	clientConfig, err := NewClientWithOptions(ClientOptions{
		Context: ctx,
		Logger:  logger.NOP(),
		Options: option.OutboundTLSOptions{
			Enabled:     true,
			Engine:      C.TLSEngineWindows,
			ServerName:  "localhost",
			Certificate: badoption.Listable[string]{serverCertificatePEM},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	conn, err := net.DialTimeout("tcp", listener.Addr().String(), windowsTLSTestTimeout)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	handshakeDone := make(chan error, 1)
	go func() {
		_, err := ClientHandshake(ctx, conn, clientConfig)
		handshakeDone <- err
	}()

	select {
	case <-clientHelloRead:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive the client hello")
	}

	cancel()

	select {
	case err := <-handshakeDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handshake did not return after cancellation")
	}
}

func TestWindowsClientHandshakeTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	// Accept but never respond.
	go func() {
		c, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer c.Close()
		time.Sleep(3 * time.Second)
	}()

	_, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")

	ctx, cancel := context.WithTimeout(context.Background(), windowsTLSTestTimeout)
	defer cancel()

	clientConfig, err := NewClientWithOptions(ClientOptions{
		Context: ctx,
		Logger:  logger.NOP(),
		Options: option.OutboundTLSOptions{
			Enabled:          true,
			Engine:           C.TLSEngineWindows,
			ServerName:       "localhost",
			HandshakeTimeout: badoption.Duration(300 * time.Millisecond),
			Certificate:      badoption.Listable[string]{serverCertificatePEM},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	conn, err := net.DialTimeout("tcp", listener.Addr().String(), windowsTLSTestTimeout)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	start := time.Now()
	_, err = ClientHandshake(ctx, conn, clientConfig)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected handshake to time out")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("handshake took %v, expected ~300ms timeout", elapsed)
	}
}

func TestWindowsClientRoundtrip(t *testing.T) {
	clientConn, serverDone := startWindowsEchoServer(t, stdtls.VersionTLS12)
	defer clientConn.Close()

	_, err := clientConn.Write([]byte("ping"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	reply := make([]byte, 4)
	_, err = io.ReadFull(clientConn, reply)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(reply) != "ping" {
		t.Fatalf("unexpected reply: %q", string(reply))
	}

	clientConn.Close()
	<-serverDone
}

func TestWindowsClientRoundtripTLS13(t *testing.T) {
	clientConn, serverDone := startWindowsEchoServer(t, stdtls.VersionTLS13)
	defer clientConn.Close()

	payload := []byte("hello tls 1.3")
	_, err := clientConn.Write(payload)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	reply := make([]byte, len(payload))
	_, err = io.ReadFull(clientConn, reply)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(payload, reply) {
		t.Fatalf("unexpected reply: %q", string(reply))
	}

	clientConn.Close()
	<-serverDone
}

func TestWindowsClientReadBuffer(t *testing.T) {
	payload := []byte("windows tls read buffer payload")
	clientConn, serverErr := startWindowsPayloadServer(t, stdtls.VersionTLS12, payload)
	defer clientConn.Close()

	const (
		frontHeadroom = 8
		rearHeadroom  = 8
	)
	buffer := buf.NewSize(len(payload) + frontHeadroom + rearHeadroom)
	defer buffer.Release()
	buffer.Resize(frontHeadroom, 0)
	buffer.Reserve(rearHeadroom)

	err := clientConn.ReadBuffer(buffer)
	if err != nil {
		t.Fatalf("ReadBuffer: %v", err)
	}
	if buffer.Start() != frontHeadroom {
		t.Fatalf("expected front headroom %d, got %d", frontHeadroom, buffer.Start())
	}
	if !bytes.Equal(buffer.Bytes(), payload) {
		t.Fatalf("unexpected payload: %q", string(buffer.Bytes()))
	}
	if err = <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestWindowsClientWriteBuffer(t *testing.T) {
	clientConn, serverDone := startWindowsEchoEngineServer(t, stdtls.VersionTLS12)
	defer clientConn.Close()

	payload := []byte("windows tls write buffer payload")
	buffer := buf.NewSize(len(payload))
	_, err := buffer.Write(payload)
	if err != nil {
		t.Fatal(err)
	}

	err = clientConn.WriteBuffer(buffer)
	if err != nil {
		t.Fatalf("WriteBuffer: %v", err)
	}
	if buffer.RawCap() != 0 {
		t.Fatalf("expected WriteBuffer to release buffer, raw cap %d", buffer.RawCap())
	}

	reply := make([]byte, len(payload))
	_, err = io.ReadFull(clientConn, reply)
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if !bytes.Equal(reply, payload) {
		t.Fatalf("unexpected echo: %q", string(reply))
	}

	clientConn.Close()
	<-serverDone
}

func TestWindowsClientCreateReadWaiter(t *testing.T) {
	payload := []byte("windows tls read waiter payload")
	clientConn, serverErr := startWindowsPayloadServer(t, stdtls.VersionTLS12, payload)
	defer clientConn.Close()

	readWaiter, created := bufio.CreateReadWaiter(clientConn)
	if !created {
		t.Fatal("expected read waiter")
	}
	readWaiter.InitializeReadWaiter(N.ReadWaitOptions{
		FrontHeadroom: 7,
		RearHeadroom:  5,
		MTU:           len(payload),
	})

	buffer, err := readWaiter.WaitReadBuffer()
	if err != nil {
		t.Fatalf("WaitReadBuffer: %v", err)
	}
	defer buffer.Release()
	if buffer.Start() != 7 {
		t.Fatalf("expected front headroom 7, got %d", buffer.Start())
	}
	if buffer.FreeLen() < 5 {
		t.Fatalf("expected rear headroom at least 5, got %d", buffer.FreeLen())
	}
	if !bytes.Equal(buffer.Bytes(), payload) {
		t.Fatalf("unexpected payload: %q", string(buffer.Bytes()))
	}
	if err = <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestWindowsClientCreateReadWaiterFallback(t *testing.T) {
	tlsConn := newTestWindowsTLSConn(&windowsTestIOConn{})
	_, created := tlsConn.CreateReadWaiter()
	if created {
		t.Fatal("expected read waiter fallback")
	}
}

func TestWindowsClientTLS13PostHandshakeConcurrentWrite(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	const payloadSize = 4 << 20
	const prefixSize = 32 << 10
	reply := []byte("tls13 post-handshake reply")

	serverErr := make(chan error, 1)
	prefixRead := make(chan struct{})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErr <- acceptErr
			return
		}
		defer conn.Close()

		tlsConn := stdtls.Server(conn, &stdtls.Config{
			Certificates: []stdtls.Certificate{serverCertificate},
			MinVersion:   stdtls.VersionTLS13,
			MaxVersion:   stdtls.VersionTLS13,
		})
		defer tlsConn.Close()

		err := tlsConn.SetDeadline(time.Now().Add(2 * windowsTLSTestTimeout))
		if err != nil {
			serverErr <- err
			return
		}
		err = tlsConn.Handshake()
		if err != nil {
			serverErr <- err
			return
		}
		prefix := make([]byte, prefixSize)
		_, err = io.ReadFull(tlsConn, prefix)
		if err != nil {
			serverErr <- err
			return
		}
		close(prefixRead)
		_, err = tlsConn.Write(reply)
		if err != nil {
			serverErr <- err
			return
		}
		_, err = io.Copy(io.Discard, io.LimitReader(tlsConn, int64(payloadSize-prefixSize)))
		if err != nil {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	clientConn, err := newWindowsTestClientConn(t, listener.Addr().String(), option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.3",
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	payload := make([]byte, payloadSize)
	for index := range payload {
		payload[index] = byte(index % 251)
	}
	writeDone := make(chan error, 1)
	go func() {
		_, err := clientConn.Write(payload)
		writeDone <- err
	}()

	select {
	case <-prefixRead:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not observe the client write")
	}

	replyBuffer := make([]byte, len(reply))
	_, err = io.ReadFull(clientConn, replyBuffer)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(reply, replyBuffer) {
		t.Fatalf("unexpected reply: %q", string(replyBuffer))
	}

	writeErr := <-writeDone
	if writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}
	serverErrValue := <-serverErr
	if serverErrValue != nil {
		t.Fatal(serverErrValue)
	}
}

func TestWindowsClientLargeMessage(t *testing.T) {
	clientConn, serverDone := startWindowsEchoServer(t, stdtls.VersionTLS12)
	defer clientConn.Close()

	// 1 MiB exercises multiple TLS records and the chunking logic.
	// Writes must run concurrently with reads to avoid TCP-buffer deadlock.
	payload := make([]byte, 1<<20)
	for index := range payload {
		payload[index] = byte(index % 251)
	}
	writeErr := make(chan error, 1)
	go func() {
		_, err := clientConn.Write(payload)
		writeErr <- err
	}()

	reply := make([]byte, len(payload))
	_, err := io.ReadFull(clientConn, reply)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	writeResult := <-writeErr
	if writeResult != nil {
		t.Fatalf("write: %v", writeResult)
	}
	if !bytes.Equal(payload, reply) {
		t.Fatal("payload mismatch after round-trip")
	}

	clientConn.Close()
	<-serverDone
}

func TestWindowsClientFullDuplexLargePayload(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	const payloadSize = 2 << 20
	clientPayload := make([]byte, payloadSize)
	serverPayload := make([]byte, payloadSize)
	for index := range clientPayload {
		clientPayload[index] = byte(index % 251)
		serverPayload[index] = byte((index + 97) % 251)
	}

	serverErr := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErr <- acceptErr
			return
		}
		defer conn.Close()

		tlsConn := stdtls.Server(conn, &stdtls.Config{
			Certificates: []stdtls.Certificate{serverCertificate},
			MinVersion:   stdtls.VersionTLS12,
			MaxVersion:   stdtls.VersionTLS12,
		})
		defer tlsConn.Close()
		err := tlsConn.SetDeadline(time.Now().Add(2 * windowsTLSTestTimeout))
		if err != nil {
			serverErr <- err
			return
		}
		err = tlsConn.Handshake()
		if err != nil {
			serverErr <- err
			return
		}

		readDone := make(chan error, 1)
		writeDone := make(chan error, 1)
		go func() {
			received := make([]byte, len(clientPayload))
			_, readErr := io.ReadFull(tlsConn, received)
			if readErr == nil && !bytes.Equal(received, clientPayload) {
				readErr = errors.New("client payload mismatch")
			}
			readDone <- readErr
		}()
		go func() {
			_, writeErr := tlsConn.Write(serverPayload)
			writeDone <- writeErr
		}()
		if readErr := <-readDone; readErr != nil {
			serverErr <- readErr
			return
		}
		serverErr <- <-writeDone
	}()

	clientConn, err := newWindowsTestEngineConn(t, listener.Addr().String(), option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.2",
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	writeDone := make(chan error, 1)
	go func() {
		n, writeErr := clientConn.Write(clientPayload)
		if writeErr == nil && n != len(clientPayload) {
			writeErr = io.ErrShortWrite
		}
		writeDone <- writeErr
	}()

	reply := make([]byte, len(serverPayload))
	_, err = io.ReadFull(clientConn, reply)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(reply, serverPayload) {
		t.Fatal("server payload mismatch")
	}
	if writeErr := <-writeDone; writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}
	if err = <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestWindowsClientMultipleRoundtrips(t *testing.T) {
	clientConn, serverDone := startWindowsEchoServer(t, stdtls.VersionTLS12)
	defer clientConn.Close()

	for i := 0; i < 100; i++ {
		payload := []byte("msg" + string(rune('A'+(i%26))))
		_, err := clientConn.Write(payload)
		if err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		reply := make([]byte, len(payload))
		_, err = io.ReadFull(clientConn, reply)
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		if !bytes.Equal(payload, reply) {
			t.Fatalf("iteration %d: expected %q got %q", i, payload, reply)
		}
	}

	clientConn.Close()
	<-serverDone
}

func TestWindowsClientConcurrentReadWrite(t *testing.T) {
	clientConn, serverDone := startWindowsEchoServer(t, stdtls.VersionTLS12)
	defer clientConn.Close()

	const messageCount = 200
	const messageSize = 64
	payloads := make([][]byte, messageCount)
	for i := range payloads {
		buffer := make([]byte, messageSize)
		for j := range buffer {
			buffer[j] = byte(i + j)
		}
		payloads[i] = buffer
	}

	readErr := make(chan error, 1)
	readBack := make(chan []byte, messageCount)
	go func() {
		for i := 0; i < messageCount; i++ {
			reply := make([]byte, messageSize)
			_, err := io.ReadFull(clientConn, reply)
			if err != nil {
				readErr <- err
				return
			}
			readBack <- reply
		}
		readErr <- nil
	}()

	for i, payload := range payloads {
		_, err := clientConn.Write(payload)
		if err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	readResult := <-readErr
	if readResult != nil {
		t.Fatal(readResult)
	}
	for i := 0; i < messageCount; i++ {
		got := <-readBack
		if !bytes.Equal(payloads[i], got) {
			t.Fatalf("iteration %d: payload mismatch", i)
		}
	}

	clientConn.Close()
	<-serverDone
}

func TestWindowsClientServerCloseReturnsEOF(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		tlsConn := stdtls.Server(conn, &stdtls.Config{
			Certificates: []stdtls.Certificate{serverCertificate},
			MinVersion:   stdtls.VersionTLS12,
			MaxVersion:   stdtls.VersionTLS12,
		})
		_ = tlsConn.Handshake()
		// Send close_notify then exit.
		_ = tlsConn.Close()
	}()

	clientConn, err := newWindowsTestClientConn(t, listener.Addr().String(), option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.2",
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	buffer := make([]byte, 16)
	_, err = clientConn.Read(buffer)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF, got %v", err)
	}
	<-done
}

func TestWindowsClientCloseUnblocksRead(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	_, serverAddress := startWindowsTLSSilentServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS12,
	})

	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.2",
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	readDone := make(chan error, 1)
	go func() {
		buffer := make([]byte, 16)
		_, err := clientConn.Read(buffer)
		readDone <- err
	}()

	time.Sleep(100 * time.Millisecond)
	clientConn.Close()

	select {
	case err := <-readDone:
		if err == nil {
			t.Fatal("expected Read to return an error after Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Read did not return within 2s after Close")
	}
}

func TestWindowsClientReadAfterCloseReturnsError(t *testing.T) {
	clientConn, serverDone := startWindowsEchoServer(t, stdtls.VersionTLS12)
	clientConn.Close()
	<-serverDone

	buffer := make([]byte, 16)
	_, err := clientConn.Read(buffer)
	if err == nil {
		t.Fatal("expected Read after Close to return error")
	}
}

func TestWindowsClientReadAfterCloseDoesNotServeBufferedPlaintext(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	serverDone := make(chan struct{})
	serverErr := make(chan error, 1)
	payload := bytes.Repeat([]byte("buffered plaintext "), 32)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErr <- acceptErr
			return
		}
		defer conn.Close()

		tlsConn := stdtls.Server(conn, &stdtls.Config{
			Certificates: []stdtls.Certificate{serverCertificate},
			MinVersion:   stdtls.VersionTLS12,
			MaxVersion:   stdtls.VersionTLS12,
		})
		defer tlsConn.Close()

		err := tlsConn.SetDeadline(time.Now().Add(windowsTLSTestTimeout))
		if err != nil {
			serverErr <- err
			return
		}
		err = tlsConn.Handshake()
		if err != nil {
			serverErr <- err
			return
		}
		_, err = tlsConn.Write(payload)
		if err != nil {
			serverErr <- err
			return
		}
		<-serverDone
		serverErr <- nil
	}()

	clientConn, err := newWindowsTestClientConn(t, listener.Addr().String(), option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.2",
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}

	buffer := make([]byte, 8)
	n, err := clientConn.Read(buffer)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}
	if n != len(buffer) {
		t.Fatalf("expected first read to fill the buffer, got %d", n)
	}

	clientConn.Close()
	close(serverDone)

	_, err = clientConn.Read(make([]byte, len(payload)))
	if !errors.Is(err, net.ErrClosed) {
		t.Fatalf("expected net.ErrClosed, got %v", err)
	}
	serverErrValue := <-serverErr
	if serverErrValue != nil {
		t.Fatal(serverErrValue)
	}
}

func TestWindowsClientWriteAfterCloseReturnsError(t *testing.T) {
	clientConn, serverDone := startWindowsEchoServer(t, stdtls.VersionTLS12)
	clientConn.Close()
	<-serverDone

	_, err := clientConn.Write([]byte("after close"))
	if err == nil {
		t.Fatal("expected Write after Close to return error")
	}
}

func TestWindowsClientReadDeadline(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	serverDone, serverAddress := startWindowsTLSSilentServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS12,
		MaxVersion:   stdtls.VersionTLS12,
	})

	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.2",
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()
	defer close(serverDone)

	err = clientConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}

	readDone := make(chan error, 1)
	buffer := make([]byte, 64)
	go func() {
		_, readErr := clientConn.Read(buffer)
		readDone <- readErr
	}()

	select {
	case readErr := <-readDone:
		if !errors.Is(readErr, os.ErrDeadlineExceeded) {
			t.Fatalf("expected os.ErrDeadlineExceeded, got %v", readErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Read did not return within 2s after deadline")
	}
}

func TestWindowsClientSetReadDeadlinePreExpired(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	serverDone, serverAddress := startWindowsTLSSilentServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS12,
		MaxVersion:   stdtls.VersionTLS12,
	})

	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.2",
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()
	defer close(serverDone)

	err = clientConn.SetReadDeadline(time.Now().Add(-time.Second))
	if err != nil {
		t.Fatalf("SetReadDeadline past: %v", err)
	}

	buffer := make([]byte, 16)
	_, err = clientConn.Read(buffer)
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("expected os.ErrDeadlineExceeded, got %v", err)
	}

	// Clearing the deadline must restore normal blocking behaviour.
	err = clientConn.SetReadDeadline(time.Time{})
	if err != nil {
		t.Fatalf("SetReadDeadline zero: %v", err)
	}
	err = clientConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if err != nil {
		t.Fatalf("SetReadDeadline future: %v", err)
	}
	start := time.Now()
	_, err = clientConn.Read(buffer)
	elapsed := time.Since(start)
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("expected os.ErrDeadlineExceeded after re-arm, got %v", err)
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("Read returned too fast (%v), pre-expired flag leaked", elapsed)
	}
}

func TestWindowsClientSetDeadlinePropagatesToRawConn(t *testing.T) {
	rawConn := &windowsTestDeadlineConn{}
	tlsConn := newTestWindowsTLSConn(rawConn)

	deadline := time.Now().Add(time.Second)
	err := tlsConn.SetDeadline(deadline)
	if err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}

	readDeadlines := rawConn.recordedReadDeadlines()
	if len(readDeadlines) != 1 {
		t.Fatalf("expected 1 read deadline update, got %d", len(readDeadlines))
	}
	if !readDeadlines[0].Equal(deadline) {
		t.Fatalf("expected read deadline %v, got %v", deadline, readDeadlines[0])
	}

	writeDeadlines := rawConn.recordedWriteDeadlines()
	if len(writeDeadlines) != 1 {
		t.Fatalf("expected 1 write deadline update, got %d", len(writeDeadlines))
	}
	if !writeDeadlines[0].Equal(deadline) {
		t.Fatalf("expected write deadline %v, got %v", deadline, writeDeadlines[0])
	}
}

func TestWindowsClientSetReadDeadlineCancelsBlockedRead(t *testing.T) {
	rawConn := &windowsTestDeadlineConn{
		readCalled: make(chan struct{}),
	}
	tlsConn := newTestWindowsTLSConn(rawConn)
	tlsConn.readScratch = make([]byte, 16)

	readErrCh := make(chan error, 1)
	go func() {
		_, err := tlsConn.Read(make([]byte, 1))
		readErrCh <- err
	}()

	select {
	case <-rawConn.readCalled:
	case <-time.After(time.Second):
		t.Fatal("Read did not reach the raw connection")
	}

	deadline := time.Now().Add(150 * time.Millisecond)
	err := tlsConn.SetReadDeadline(deadline)
	if err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}

	select {
	case err = <-readErrCh:
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("expected os.ErrDeadlineExceeded, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Read did not return after SetReadDeadline")
	}

	readDeadlines := rawConn.recordedReadDeadlines()
	if len(readDeadlines) != 1 {
		t.Fatalf("expected 1 read deadline update, got %d", len(readDeadlines))
	}
	if !readDeadlines[0].Equal(deadline) {
		t.Fatalf("expected read deadline %v, got %v", deadline, readDeadlines[0])
	}
}

func TestWindowsClientSetWriteDeadlineCancelsBlockedWrite(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	serverDone, serverAddress := startWindowsTLSSilentServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS12,
		MaxVersion:   stdtls.VersionTLS12,
	})
	defer close(serverDone)

	tlsConn, err := newWindowsTestEngineConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.2",
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}

	originalRawConn := tlsConn.rawConn
	rawConn := &windowsTestDeadlineConn{
		writeCalled: make(chan struct{}),
	}
	tlsConn.rawConn = rawConn
	t.Cleanup(func() {
		_ = originalRawConn.Close()
		_ = tlsConn.Close()
	})

	writeErrCh := make(chan error, 1)
	go func() {
		_, err := tlsConn.Write([]byte("ping"))
		writeErrCh <- err
	}()

	select {
	case <-rawConn.writeCalled:
	case <-time.After(time.Second):
		t.Fatal("Write did not reach the raw connection")
	}

	deadline := time.Now().Add(150 * time.Millisecond)
	err = tlsConn.SetWriteDeadline(deadline)
	if err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}

	select {
	case err = <-writeErrCh:
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("expected os.ErrDeadlineExceeded, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Write did not return after SetWriteDeadline")
	}

	writeDeadlines := rawConn.recordedWriteDeadlines()
	if len(writeDeadlines) != 1 {
		t.Fatalf("expected 1 write deadline update, got %d", len(writeDeadlines))
	}
	if !writeDeadlines[0].Equal(deadline) {
		t.Fatalf("expected write deadline %v, got %v", deadline, writeDeadlines[0])
	}
}

func TestWindowsClientPostHandshakeReplyUsesReadDeadline(t *testing.T) {
	rawConn := &windowsTestDeadlineConn{}
	tlsConn := newTestWindowsTLSConn(rawConn)

	readDeadline := time.Now().Add(150 * time.Millisecond)
	err := tlsConn.SetReadDeadline(readDeadline)
	if err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}

	start := time.Now()
	err = tlsConn.writePostHandshakeReply([]byte("reply"))
	elapsed := time.Since(start)
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("expected os.ErrDeadlineExceeded, got %v", err)
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("post-handshake write returned too fast: %v", elapsed)
	}

	deadlines := rawConn.recordedWriteDeadlines()
	if len(deadlines) != 2 {
		t.Fatalf("expected 2 write deadline updates, got %d", len(deadlines))
	}
	if !deadlines[0].Equal(readDeadline) {
		t.Fatalf("expected first write deadline %v, got %v", readDeadline, deadlines[0])
	}
	if !deadlines[1].IsZero() {
		t.Fatalf("expected write deadline cleanup, got %v", deadlines[1])
	}
}

func TestWindowsClientPostHandshakeReplyPreExpiredReadDeadline(t *testing.T) {
	rawConn := &windowsTestDeadlineConn{}
	tlsConn := newTestWindowsTLSConn(rawConn)

	err := tlsConn.SetReadDeadline(time.Now().Add(-time.Second))
	if err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}

	start := time.Now()
	err = tlsConn.writePostHandshakeReply([]byte("reply"))
	elapsed := time.Since(start)
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("expected os.ErrDeadlineExceeded, got %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("pre-expired post-handshake write returned too slowly: %v", elapsed)
	}

	deadlines := rawConn.recordedWriteDeadlines()
	if len(deadlines) != 0 {
		t.Fatalf("expected no write deadline update for pre-expired read deadline, got %d", len(deadlines))
	}
}

func TestDriveStepsPreservesBufferedHandshakeBytes(t *testing.T) {
	scratch := make([]byte, 8)
	copy(scratch, "abc")

	readCalls := 0
	stepCalls := 0
	leftover, err := driveSteps(
		scratch[:3],
		func(input []byte) (schannel.StepResult, error) {
			stepCalls++
			switch stepCalls {
			case 1:
				if string(input) != "abc" {
					t.Fatalf("first step input = %q, want %q", input, "abc")
				}
				return schannel.StepResult{Incomplete: true}, nil
			case 2:
				if string(input) != "abcdef" {
					t.Fatalf("second step input = %q, want %q", input, "abcdef")
				}
				return schannel.StepResult{Consumed: len(input), Done: true}, nil
			default:
				t.Fatalf("unexpected step call %d", stepCalls)
				return schannel.StepResult{}, nil
			}
		},
		func() ([]byte, error) {
			readCalls++
			copy(scratch, "def")
			return scratch[:3], nil
		},
		func([]byte) error { return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if readCalls != 1 {
		t.Fatalf("readMore called %d times, want 1", readCalls)
	}
	if len(leftover) != 0 {
		t.Fatalf("leftover = %q, want empty", leftover)
	}
}

func TestWindowsTLSRawReadEOFAtRecordBoundary(t *testing.T) {
	rawConn := &windowsTestIOConn{readErr: io.EOF}
	_, err := readTLSRaw(rawConn, make([]byte, 16), false)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}

func TestWindowsTLSRawReadEOFWithPendingRecord(t *testing.T) {
	rawConn := &windowsTestIOConn{readErr: io.EOF}
	_, err := readTLSRaw(rawConn, make([]byte, 16), true)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
	}
}

func TestWindowsClientPostHandshakeReplyWaitsForWriteAccess(t *testing.T) {
	rawConn := &windowsTestWriteGateConn{
		writeCalled:  make(chan struct{}),
		releaseWrite: make(chan struct{}),
	}
	tlsConn := newTestWindowsTLSConn(rawConn)

	tlsConn.writeAccess.Lock()
	errCh := make(chan error, 1)
	go func() {
		errCh <- tlsConn.writePostHandshakeReply([]byte("reply"))
	}()

	select {
	case <-rawConn.writeCalled:
		t.Fatal("post-handshake write bypassed writeAccess")
	case <-time.After(100 * time.Millisecond):
	}

	tlsConn.writeAccess.Unlock()

	select {
	case <-rawConn.writeCalled:
	case <-time.After(time.Second):
		t.Fatal("post-handshake write did not resume after writeAccess release")
	}

	close(rawConn.releaseWrite)
	err := <-errCh
	if err != nil {
		t.Fatal(err)
	}
}

func TestWindowsClientPostHandshakeWritePreemptsNewWrite(t *testing.T) {
	tlsConn := newTestWindowsTLSConn(&windowsTestIOConn{})
	err := tlsConn.beginWrite()
	if err != nil {
		t.Fatal(err)
	}

	postHandshakeReady := make(chan error, 1)
	go func() {
		postHandshakeReady <- tlsConn.beginPostHandshakeWrite()
	}()

	deadline := time.After(time.Second)
	for {
		tlsConn.writeState.Lock()
		pending := tlsConn.postHandshake
		tlsConn.writeState.Unlock()
		if pending {
			break
		}
		select {
		case <-deadline:
			t.Fatal("post-handshake write did not become pending")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	writeReady := make(chan error, 1)
	go func() {
		writeReady <- tlsConn.beginWrite()
	}()

	tlsConn.finishWrite()

	select {
	case err = <-postHandshakeReady:
		if err != nil {
			t.Fatal(err)
		}
	case err = <-writeReady:
		t.Fatalf("new write preempted post-handshake write: %v", err)
	case <-time.After(time.Second):
		t.Fatal("post-handshake write did not resume")
	}

	select {
	case err = <-writeReady:
		t.Fatalf("new write acquired before post-handshake finished: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	tlsConn.finishPostHandshakeWrite()
	select {
	case err = <-writeReady:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("new write did not resume after post-handshake write")
	}
	tlsConn.finishWrite()
}

func TestWindowsClientPostHandshakeReplyErrorClosesConn(t *testing.T) {
	rawConn := &windowsTestIOConn{
		writeErr: os.ErrDeadlineExceeded,
		writeN:   1,
	}
	tlsConn := newTestWindowsTLSConn(rawConn)

	err := tlsConn.writePostHandshakeReply([]byte("reply"))
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("expected os.ErrDeadlineExceeded, got %v", err)
	}
	if !rawConn.isClosed() {
		t.Fatal("expected raw conn to be closed")
	}
	if !tlsConn.isClosed() {
		t.Fatal("expected tls conn to be closed")
	}
}

func TestWindowsClientWriteErrorClosesConn(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	serverDone, serverAddress := startWindowsTLSSilentServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS12,
		MaxVersion:   stdtls.VersionTLS12,
	})
	defer close(serverDone)

	tlsConn, err := newWindowsTestEngineConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.2",
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}

	originalRawConn := tlsConn.rawConn
	rawConn := &windowsTestIOConn{
		writeErr: os.ErrDeadlineExceeded,
		writeN:   1,
	}
	tlsConn.rawConn = rawConn
	t.Cleanup(func() {
		_ = originalRawConn.Close()
		_ = tlsConn.Close()
	})

	_, err = tlsConn.Write([]byte("ping"))
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("expected os.ErrDeadlineExceeded, got %v", err)
	}
	if !rawConn.isClosed() {
		t.Fatal("expected raw conn to be closed")
	}
	if !tlsConn.isClosed() {
		t.Fatal("expected tls conn to be closed")
	}

	_, err = tlsConn.Write([]byte("again"))
	if !errors.Is(err, net.ErrClosed) {
		t.Fatalf("expected net.ErrClosed on second write, got %v", err)
	}
	if rawConn.totalWriteCalls() != 1 {
		t.Fatalf("expected exactly 1 raw write, got %d", rawConn.totalWriteCalls())
	}

	_, err = tlsConn.Read(make([]byte, 1))
	if !errors.Is(err, net.ErrClosed) {
		t.Fatalf("expected net.ErrClosed on read after write failure, got %v", err)
	}
}

func TestWindowsClientConnectionStateFields(t *testing.T) {
	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	_, serverAddress := startWindowsTLSTestServer(t, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS12,
		NextProtos:   []string{"h2"},
	})

	clientConn, err := newWindowsTestClientConn(t, serverAddress, option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  "1.2",
		ALPN:        badoption.Listable[string]{"h2"},
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	state := clientConn.ConnectionState()
	if state.ServerName != "localhost" {
		t.Errorf("ServerName: expected localhost, got %q", state.ServerName)
	}
	if state.NegotiatedProtocol != "h2" {
		t.Errorf("NegotiatedProtocol: expected h2, got %q", state.NegotiatedProtocol)
	}
	if !state.HandshakeComplete {
		t.Error("HandshakeComplete: expected true")
	}
	if state.Version < stdtls.VersionTLS12 || state.Version > stdtls.VersionTLS13 {
		t.Errorf("Version: expected TLS 1.2–1.3, got %x", state.Version)
	}
	if len(state.PeerCertificates) == 0 {
		t.Fatal("PeerCertificates: expected at least one certificate")
	}
	// CipherSuite may be 0 when the Schannel name does not map to a Go
	// constant; just ensure it's consistent with the protocol.
	if state.Version == stdtls.VersionTLS13 && state.CipherSuite != 0 {
		switch state.CipherSuite {
		case stdtls.TLS_AES_128_GCM_SHA256, stdtls.TLS_AES_256_GCM_SHA384, stdtls.TLS_CHACHA20_POLY1305_SHA256:
		default:
			t.Errorf("unexpected TLS 1.3 cipher suite: %x", state.CipherSuite)
		}
	}
}

func TestWindowsClientNetConnReturnsUnderlying(t *testing.T) {
	clientConn, serverDone := startWindowsEchoServer(t, stdtls.VersionTLS12)
	defer func() { <-serverDone }()
	defer clientConn.Close()

	underlying := clientConn.NetConn()
	if _, isTCP := underlying.(*net.TCPConn); !isTCP {
		t.Fatalf("NetConn returned %T, expected *net.TCPConn", underlying)
	}
}

func TestNewWindowsClientMissingServerName(t *testing.T) {
	_, err := NewClientWithOptions(ClientOptions{
		Context: context.Background(),
		Logger:  logger.NOP(),
		Options: option.OutboundTLSOptions{
			Enabled: true,
			Engine:  C.TLSEngineWindows,
		},
	})
	if err == nil {
		t.Fatal("expected missing server_name error")
	}
}

func TestNewWindowsClientInsecureAllowsMissingServerName(t *testing.T) {
	_, err := NewClientWithOptions(ClientOptions{
		Context: context.Background(),
		Logger:  logger.NOP(),
		Options: option.OutboundTLSOptions{
			Enabled:  true,
			Engine:   C.TLSEngineWindows,
			Insecure: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWindowsClientConfigSTDConfigReturnsError(t *testing.T) {
	config, err := NewClientWithOptions(ClientOptions{
		Context: context.Background(),
		Logger:  logger.NOP(),
		Options: option.OutboundTLSOptions{
			Enabled:    true,
			Engine:     C.TLSEngineWindows,
			ServerName: "localhost",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = config.STDConfig()
	if err == nil {
		t.Fatal("expected STDConfig() to return error for Windows engine")
	}
	if !strings.Contains(err.Error(), "system TLS engine") {
		t.Fatalf("expected error to name the engine, got %q", err.Error())
	}
}

func TestWindowsClientConfigClientReturnsErrInvalid(t *testing.T) {
	config, err := NewClientWithOptions(ClientOptions{
		Context: context.Background(),
		Logger:  logger.NOP(),
		Options: option.OutboundTLSOptions{
			Enabled:    true,
			Engine:     C.TLSEngineWindows,
			ServerName: "localhost",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = config.Client(nil)
	if !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("expected os.ErrInvalid, got %v", err)
	}
}

func TestWindowsClientConfigClone(t *testing.T) {
	config, err := NewClientWithOptions(ClientOptions{
		Context: context.Background(),
		Logger:  logger.NOP(),
		Options: option.OutboundTLSOptions{
			Enabled:    true,
			Engine:     C.TLSEngineWindows,
			ServerName: "localhost",
			ALPN:       badoption.Listable[string]{"h2", "http/1.1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	clone := config.Clone()

	// Mutating the clone must not affect the original.
	clone.SetServerName("other")
	clone.SetNextProtos([]string{"h3"})
	if config.ServerName() == "other" {
		t.Error("Clone shares server name with original")
	}
	if len(config.NextProtos()) != 2 {
		t.Error("Clone shares ALPN slice with original")
	}
}

func TestValidateWindowsTLSOptionsRejections(t *testing.T) {
	cases := []struct {
		name    string
		options option.OutboundTLSOptions
		needle  string
	}{
		{"reality", option.OutboundTLSOptions{
			Enabled: true, Engine: C.TLSEngineWindows, ServerName: "x",
			Reality: &option.OutboundRealityOptions{Enabled: true, ShortID: "abc"},
		}, "reality"},
		{"utls", option.OutboundTLSOptions{
			Enabled: true, Engine: C.TLSEngineWindows, ServerName: "x",
			UTLS: &option.OutboundUTLSOptions{Enabled: true},
		}, "utls"},
		{"ech", option.OutboundTLSOptions{
			Enabled: true, Engine: C.TLSEngineWindows, ServerName: "x",
			ECH: &option.OutboundECHOptions{Enabled: true},
		}, "ech"},
		{"disable_sni", option.OutboundTLSOptions{
			Enabled: true, Engine: C.TLSEngineWindows, ServerName: "x",
			DisableSNI: true,
		}, "disable_sni"},
		{"cipher_suites", option.OutboundTLSOptions{
			Enabled: true, Engine: C.TLSEngineWindows, ServerName: "x",
			CipherSuites: []string{"TLS_AES_128_GCM_SHA256"},
		}, "cipher_suites"},
		{"curve_preferences", option.OutboundTLSOptions{
			Enabled: true, Engine: C.TLSEngineWindows, ServerName: "x",
			CurvePreferences: []option.CurvePreference{option.CurvePreference(29)},
		}, "curve_preferences"},
		{"client_certificate", option.OutboundTLSOptions{
			Enabled: true, Engine: C.TLSEngineWindows, ServerName: "x",
			ClientCertificate: badoption.Listable[string]{"pem"},
		}, "client certificate"},
		{"fragment", option.OutboundTLSOptions{
			Enabled: true, Engine: C.TLSEngineWindows, ServerName: "x",
			Fragment: true,
		}, "tls fragment"},
		{"record_fragment", option.OutboundTLSOptions{
			Enabled: true, Engine: C.TLSEngineWindows, ServerName: "x",
			RecordFragment: true,
		}, "tls fragment"},
		{"kernel_tx", option.OutboundTLSOptions{
			Enabled: true, Engine: C.TLSEngineWindows, ServerName: "x",
			KernelTx: true,
		}, "ktls"},
		{"kernel_rx", option.OutboundTLSOptions{
			Enabled: true, Engine: C.TLSEngineWindows, ServerName: "x",
			KernelRx: true,
		}, "ktls"},
		{"spoof", option.OutboundTLSOptions{
			Enabled: true, Engine: C.TLSEngineWindows, ServerName: "x",
			Spoof: "decoy.example",
		}, "spoof"},
		{"pin_and_cert_conflict", option.OutboundTLSOptions{
			Enabled: true, Engine: C.TLSEngineWindows, ServerName: "x",
			Certificate:                badoption.Listable[string]{"-----BEGIN CERTIFICATE-----"},
			CertificatePublicKeySHA256: [][]byte{make([]byte, 32)},
		}, "certificate_public_key_sha256"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewClientWithOptions(ClientOptions{
				Context: context.Background(),
				Logger:  logger.NOP(),
				Options: tc.options,
			})
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.needle)
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.needle)) {
				t.Fatalf("expected error to contain %q, got %q", tc.needle, err.Error())
			}
		})
	}
}

func startWindowsTLSSilentServer(t *testing.T, tlsConfig *stdtls.Config) (chan<- struct{}, string) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	if tcpListener, isTCP := listener.(*net.TCPListener); isTCP {
		err = tcpListener.SetDeadline(time.Now().Add(windowsTLSTestTimeout))
		if err != nil {
			t.Fatal(err)
		}
	}

	done := make(chan struct{})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		deadlineErr := conn.SetDeadline(time.Now().Add(windowsTLSTestTimeout))
		if deadlineErr != nil {
			return
		}
		tlsConn := stdtls.Server(conn, tlsConfig)
		defer tlsConn.Close()
		handshakeErr := tlsConn.Handshake()
		if handshakeErr != nil {
			return
		}
		handshakeErr = conn.SetDeadline(time.Time{})
		if handshakeErr != nil {
			return
		}
		<-done
	}()
	return done, listener.Addr().String()
}

// sharedWindowsTestCertificate caches a localhost certificate so the RSA key
// generation runs once per test binary instead of once per test.
var sharedWindowsTestCertificate = sync.OnceValues(func() (stdtls.Certificate, string) {
	return generateWindowsTestCertificate("localhost")
})

func newWindowsTestCertificate(t *testing.T, serverName string) (stdtls.Certificate, string) {
	t.Helper()
	if serverName == "localhost" {
		return sharedWindowsTestCertificate()
	}
	return generateWindowsTestCertificate(serverName)
}

func generateWindowsTestCertificate(serverName string) (stdtls.Certificate, string) {
	privateKeyPEM, certificatePEM, err := GenerateCertificate(nil, nil, time.Now, serverName, time.Now().Add(time.Hour))
	if err != nil {
		panic(err)
	}
	certificate, err := stdtls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		panic(err)
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		panic(err)
	}
	certificate.Leaf = leaf
	return certificate, string(certificatePEM)
}

func publicKeyPin(t *testing.T, cert *x509.Certificate) []byte {
	t.Helper()
	pub, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(pub)
	return sum[:]
}

func startWindowsTLSTestServer(t *testing.T, tlsConfig *stdtls.Config) (<-chan windowsTLSServerResult, string) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	if tcpListener, isTCP := listener.(*net.TCPListener); isTCP {
		err = tcpListener.SetDeadline(time.Now().Add(windowsTLSTestTimeout))
		if err != nil {
			t.Fatal(err)
		}
	}

	result := make(chan windowsTLSServerResult, 1)
	go func() {
		defer close(result)

		conn, err := listener.Accept()
		if err != nil {
			result <- windowsTLSServerResult{err: err}
			return
		}
		defer conn.Close()

		err = conn.SetDeadline(time.Now().Add(windowsTLSTestTimeout))
		if err != nil {
			result <- windowsTLSServerResult{err: err}
			return
		}

		tlsConn := stdtls.Server(conn, tlsConfig)
		defer tlsConn.Close()

		err = tlsConn.Handshake()
		if err != nil {
			result <- windowsTLSServerResult{err: err}
			return
		}

		result <- windowsTLSServerResult{state: tlsConn.ConnectionState()}
	}()

	return result, listener.Addr().String()
}

func newWindowsTestClientConn(t *testing.T, serverAddress string, options option.OutboundTLSOptions) (Conn, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), windowsTLSTestTimeout)
	t.Cleanup(cancel)

	clientConfig, err := NewClientWithOptions(ClientOptions{
		Context:       ctx,
		Logger:        logger.NOP(),
		ServerAddress: "",
		Options:       options,
	})
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTimeout("tcp", serverAddress, windowsTLSTestTimeout)
	if err != nil {
		return nil, err
	}

	tlsConn, err := ClientHandshake(ctx, conn, clientConfig)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func newWindowsTestEngineConn(t *testing.T, serverAddress string, options option.OutboundTLSOptions) (*windowsTLSConn, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), windowsTLSTestTimeout)
	t.Cleanup(cancel)

	clientConfig, err := NewClientWithOptions(ClientOptions{
		Context:       ctx,
		Logger:        logger.NOP(),
		ServerAddress: "",
		Options:       options,
	})
	if err != nil {
		return nil, err
	}

	engineConfig, ok := clientConfig.(*windowsClientConfig)
	if !ok {
		return nil, errors.New("unexpected windows config type")
	}

	conn, err := net.DialTimeout("tcp", serverAddress, windowsTLSTestTimeout)
	if err != nil {
		return nil, err
	}

	tlsConn, err := engineConfig.ClientHandshake(ctx, conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	engineConn, ok := tlsConn.(*windowsTLSConn)
	if !ok {
		tlsConn.Close()
		return nil, errors.New("unexpected windows conn type")
	}
	return engineConn, nil
}

func startWindowsPayloadServer(t *testing.T, minVersion uint16, payload []byte) (*windowsTLSConn, <-chan error) {
	t.Helper()

	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	serverErr := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErr <- acceptErr
			return
		}
		defer conn.Close()

		tlsConn := stdtls.Server(conn, &stdtls.Config{
			Certificates: []stdtls.Certificate{serverCertificate},
			MinVersion:   minVersion,
			MaxVersion:   minVersion,
		})
		defer tlsConn.Close()

		handshakeErr := tlsConn.Handshake()
		if handshakeErr != nil {
			serverErr <- handshakeErr
			return
		}
		_, writeErr := tlsConn.Write(payload)
		serverErr <- writeErr
	}()

	version := "1.2"
	if minVersion == stdtls.VersionTLS13 {
		version = "1.3"
	}
	clientConn, err := newWindowsTestEngineConn(t, listener.Addr().String(), option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  version,
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	return clientConn, serverErr
}

// startWindowsEchoServer brings up a TLS echo server with a self-signed cert
// and dials an engine client against it. The returned channel closes after
// the server goroutine exits.
func startWindowsEchoServer(t *testing.T, minVersion uint16) (Conn, <-chan struct{}) {
	t.Helper()

	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()

		tlsConn := stdtls.Server(conn, &stdtls.Config{
			Certificates: []stdtls.Certificate{serverCertificate},
			MinVersion:   minVersion,
			MaxVersion:   minVersion,
		})
		defer tlsConn.Close()

		handshakeErr := tlsConn.Handshake()
		if handshakeErr != nil {
			return
		}

		buffer := make([]byte, 32*1024)
		for {
			n, readErr := tlsConn.Read(buffer)
			if n > 0 {
				_, writeErr := tlsConn.Write(buffer[:n])
				if writeErr != nil {
					return
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	version := "1.2"
	if minVersion == stdtls.VersionTLS13 {
		version = "1.3"
	}
	clientConn, err := newWindowsTestClientConn(t, listener.Addr().String(), option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  version,
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	return clientConn, done
}

func startWindowsEchoEngineServer(t *testing.T, minVersion uint16) (*windowsTLSConn, <-chan struct{}) {
	t.Helper()

	serverCertificate, serverCertificatePEM := newWindowsTestCertificate(t, "localhost")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()

		tlsConn := stdtls.Server(conn, &stdtls.Config{
			Certificates: []stdtls.Certificate{serverCertificate},
			MinVersion:   minVersion,
			MaxVersion:   minVersion,
		})
		defer tlsConn.Close()

		handshakeErr := tlsConn.Handshake()
		if handshakeErr != nil {
			return
		}

		buffer := make([]byte, 32*1024)
		for {
			n, readErr := tlsConn.Read(buffer)
			if n > 0 {
				_, writeErr := tlsConn.Write(buffer[:n])
				if writeErr != nil {
					return
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	version := "1.2"
	if minVersion == stdtls.VersionTLS13 {
		version = "1.3"
	}
	clientConn, err := newWindowsTestEngineConn(t, listener.Addr().String(), option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      C.TLSEngineWindows,
		ServerName:  "localhost",
		MinVersion:  version,
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		t.Fatal(err)
	}
	return clientConn, done
}
