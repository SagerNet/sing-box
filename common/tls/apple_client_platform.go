//go:build darwin && cgo

package tls

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework Network -framework Security

#include <stdlib.h>
#include "apple_client_platform_darwin.h"
*/
import "C"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"io"
	"math"
	"net"
	"os"
	"runtime/cgo"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/sys/unix"
)

func (c *appleClientConfig) ClientHandshake(ctx context.Context, conn net.Conn) (Conn, error) {
	tcpConn, ok := N.UnwrapReader(conn).(*net.TCPConn)
	if !ok {
		return nil, E.New("apple TLS: requires fd-backed TCP connection")
	}
	syscallConn, err := tcpConn.SyscallConn()
	if err != nil {
		return nil, E.Cause(err, "access raw connection")
	}

	var dupFD int
	controlErr := syscallConn.Control(func(fd uintptr) {
		dupFD, err = unix.Dup(int(fd))
	})
	if controlErr != nil {
		return nil, E.Cause(controlErr, "access raw connection")
	}
	if err != nil {
		return nil, E.Cause(err, "duplicate raw connection")
	}

	serverName := c.serverName
	serverNamePtr := cStringOrNil(serverName)
	defer cFree(serverNamePtr)

	alpn := strings.Join(c.nextProtos, "\n")
	alpnPtr := cStringOrNil(alpn)
	defer cFree(alpnPtr)

	anchors, err := c.resolveAnchors()
	if err != nil {
		return nil, err
	}
	var anchorsRef unsafe.Pointer
	if anchors != nil {
		anchorsRef = anchors.Ref()
	}

	var (
		hasVerifyTime       bool
		verifyTimeUnixMilli int64
	)
	if c.timeFunc != nil {
		hasVerifyTime = true
		verifyTimeUnixMilli = c.timeFunc().UnixMilli()
	}

	var errorPtr *C.char
	client := C.box_apple_tls_client_create(
		C.int(dupFD),
		serverNamePtr,
		alpnPtr,
		C.size_t(len(alpn)),
		C.uint16_t(c.minVersion),
		C.uint16_t(c.maxVersion),
		C.bool(c.insecure),
		anchorsRef,
		C.bool(c.anchorOnly),
		C.bool(hasVerifyTime),
		C.int64_t(verifyTimeUnixMilli),
		&errorPtr,
	)
	if anchors != nil {
		anchors.Release()
	}
	if client == nil {
		if errorPtr != nil {
			defer C.free(unsafe.Pointer(errorPtr))
			return nil, E.New(C.GoString(errorPtr))
		}
		return nil, E.New("apple TLS: create connection")
	}
	if err = waitAppleTLSClientReady(ctx, client); err != nil {
		C.box_apple_tls_client_cancel(client)
		C.box_apple_tls_client_free(client)
		return nil, err
	}

	var state C.box_apple_tls_state_t
	stateOK := C.box_apple_tls_client_copy_state(client, &state, &errorPtr)
	if !bool(stateOK) {
		C.box_apple_tls_client_cancel(client)
		C.box_apple_tls_client_free(client)
		if errorPtr != nil {
			defer C.free(unsafe.Pointer(errorPtr))
			return nil, E.New(C.GoString(errorPtr))
		}
		return nil, E.New("apple TLS: read metadata")
	}
	defer C.box_apple_tls_state_free(&state)

	connectionState, rawCerts, err := parseAppleTLSState(&state)
	if err != nil {
		C.box_apple_tls_client_cancel(client)
		C.box_apple_tls_client_free(client)
		return nil, err
	}
	if len(c.certificatePublicKeySHA256) > 0 {
		err = VerifyPublicKeySHA256(c.certificatePublicKeySHA256, rawCerts)
		if err != nil {
			C.box_apple_tls_client_cancel(client)
			C.box_apple_tls_client_free(client)
			return nil, err
		}
	}

	return &appleTLSConn{
		rawConn: conn,
		client:  client,
		state:   connectionState,
		closed:  make(chan struct{}),
	}, nil
}

const (
	appleTLSHandshakePollInterval = 100 * time.Millisecond
	appleTLSWriteChunkSize        = 32 * 1024
)

func waitAppleTLSClientReady(ctx context.Context, client *C.box_apple_tls_client_t) error {
	for {
		err := ctx.Err()
		if err != nil {
			C.box_apple_tls_client_cancel(client)
			return err
		}

		waitTimeout := appleTLSHandshakePollInterval
		deadline, loaded := ctx.Deadline()
		if loaded {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				C.box_apple_tls_client_cancel(client)
				err = ctx.Err()
				if err != nil {
					return err
				}
				return context.DeadlineExceeded
			}
			if remaining < waitTimeout {
				waitTimeout = remaining
			}
		}

		var errorPtr *C.char
		waitResult := C.box_apple_tls_client_wait_ready(client, C.int(timeoutFromDuration(waitTimeout)), &errorPtr)
		switch waitResult {
		case 1:
			return nil
		case -2:
			continue
		case 0:
			if errorPtr != nil {
				defer C.free(unsafe.Pointer(errorPtr))
				return E.New(C.GoString(errorPtr))
			}
			return E.New("apple TLS: handshake failed")
		default:
			return E.New("apple TLS: invalid handshake state")
		}
	}
}

type appleTLSConn struct {
	rawConn net.Conn
	client  *C.box_apple_tls_client_t
	state   tls.ConnectionState

	readAccess     sync.Mutex
	writeAccess    sync.Mutex
	stateAccess    sync.RWMutex
	closeOnce      sync.Once
	ioAccess       sync.Mutex
	ioGroup        sync.WaitGroup
	closed         chan struct{}
	readEOF        bool
	deadlineAccess sync.Mutex
	readDeadline   time.Time
	writeDeadline  time.Time
	readTimedOut   bool
	writeTimedOut  bool
}

var (
	_ N.ExtendedConn    = (*appleTLSConn)(nil)
	_ N.ReadWaitCreator = (*appleTLSConn)(nil)
)

func (c *appleTLSConn) Read(p []byte) (int, error) {
	c.readAccess.Lock()
	defer c.readAccess.Unlock()
	if c.readEOF {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}

	return c.readIntoLocked(p)
}

func (c *appleTLSConn) ReadBuffer(buffer *buf.Buffer) error {
	c.readAccess.Lock()
	defer c.readAccess.Unlock()
	if buffer.IsFull() {
		return io.ErrShortBuffer
	}
	startLen := buffer.Len()
	n, err := c.readIntoLocked(buffer.FreeBytes())
	buffer.Truncate(startLen + n)
	return err
}

func (c *appleTLSConn) readIntoLocked(p []byte) (int, error) {
	if c.readEOF {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}

	timeoutMs, err := c.prepareReadTimeout()
	if err != nil {
		return 0, err
	}

	client, err := c.acquireClient()
	if err != nil {
		return 0, err
	}
	defer c.releaseClient()

	var eof C.bool
	var errorPtr *C.char
	n := C.box_apple_tls_client_read(client, unsafe.Pointer(&p[0]), C.size_t(len(p)), C.int(timeoutMs), &eof, &errorPtr)
	switch {
	case n == -2:
		c.markReadTimedOut()
		return 0, os.ErrDeadlineExceeded
	case n >= 0:
		if bool(eof) {
			c.readEOF = true
			if n == 0 {
				return 0, io.EOF
			}
		}
		return int(n), nil
	default:
		if errorPtr != nil {
			defer C.free(unsafe.Pointer(errorPtr))
			if c.isClosed() {
				return 0, net.ErrClosed
			}
			return 0, E.New(C.GoString(errorPtr))
		}
		return 0, net.ErrClosed
	}
}

func (c *appleTLSConn) Write(p []byte) (int, error) {
	c.writeAccess.Lock()
	defer c.writeAccess.Unlock()
	if len(p) == 0 {
		return 0, nil
	}

	client, err := c.acquireClient()
	if err != nil {
		return 0, err
	}
	defer c.releaseClient()

	deadline, err := c.prepareWriteDeadline()
	if err != nil {
		return 0, err
	}
	var written int
	for written < len(p) {
		timeoutMs, expired := deadlineTimeoutMs(deadline)
		if expired {
			C.box_apple_tls_client_cancel(client)
			c.markWriteTimedOut()
			return written, os.ErrDeadlineExceeded
		}
		chunkSize := min(len(p)-written, appleTLSWriteChunkSize)
		chunk := p[written : written+chunkSize]
		var errorPtr *C.char
		n := C.box_apple_tls_client_write(client, unsafe.Pointer(&chunk[0]), C.size_t(len(chunk)), C.int(timeoutMs), &errorPtr)
		switch {
		case n == -2:
			c.markWriteTimedOut()
			return written, os.ErrDeadlineExceeded
		case n >= 0:
			written += int(n)
			if int(n) != len(chunk) {
				return written, io.ErrShortWrite
			}
			continue
		}
		return written, c.errorFromPointer(errorPtr)
	}
	return written, nil
}

func (c *appleTLSConn) WriteBuffer(buffer *buf.Buffer) error {
	defer buffer.Release()
	_, err := c.Write(buffer.Bytes())
	return err
}

func (c *appleTLSConn) CreateReadWaiter() (N.ReadWaiter, bool) {
	return &appleTLSReadWaiter{
		conn:    c,
		results: make(chan *C.box_apple_tls_read_result_t, 1),
	}, true
}

func (c *appleTLSConn) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		close(c.closed)
		C.box_apple_tls_client_cancel(c.client)
		closeErr = c.rawConn.Close()
		c.ioAccess.Lock()
		c.ioGroup.Wait()
		C.box_apple_tls_client_free(c.client)
		c.client = nil
		c.ioAccess.Unlock()
	})
	return closeErr
}

func (c *appleTLSConn) LocalAddr() net.Addr {
	return c.rawConn.LocalAddr()
}

func (c *appleTLSConn) RemoteAddr() net.Addr {
	return c.rawConn.RemoteAddr()
}

// SetDeadline installs deadlines for subsequent Read and Write calls.
//
// Deadlines only apply to subsequent Read or Write calls; an in-flight call
// does not observe later updates to its deadline. Callers that need to cancel
// an in-flight I/O must Close the connection instead.
//
// Once an active Read or Write trips its deadline, the underlying
// nw_connection is cancelled and the conn is no longer usable — callers must
// Close after a deadline error.
func (c *appleTLSConn) SetDeadline(t time.Time) error {
	c.deadlineAccess.Lock()
	c.readDeadline = t
	c.writeDeadline = t
	c.readTimedOut = false
	c.writeTimedOut = false
	c.deadlineAccess.Unlock()
	return nil
}

func (c *appleTLSConn) SetReadDeadline(t time.Time) error {
	c.deadlineAccess.Lock()
	c.readDeadline = t
	c.readTimedOut = false
	c.deadlineAccess.Unlock()
	return nil
}

func (c *appleTLSConn) SetWriteDeadline(t time.Time) error {
	c.deadlineAccess.Lock()
	c.writeDeadline = t
	c.writeTimedOut = false
	c.deadlineAccess.Unlock()
	return nil
}

func (c *appleTLSConn) prepareReadTimeout() (int, error) {
	c.deadlineAccess.Lock()
	defer c.deadlineAccess.Unlock()
	if c.readTimedOut {
		return 0, os.ErrDeadlineExceeded
	}
	timeoutMs, expired := deadlineTimeoutMs(c.readDeadline)
	if expired {
		c.readTimedOut = true
		return 0, os.ErrDeadlineExceeded
	}
	return timeoutMs, nil
}

func (c *appleTLSConn) prepareWriteDeadline() (time.Time, error) {
	c.deadlineAccess.Lock()
	defer c.deadlineAccess.Unlock()
	if c.writeTimedOut {
		return time.Time{}, os.ErrDeadlineExceeded
	}
	_, expired := deadlineTimeoutMs(c.writeDeadline)
	if expired {
		c.writeTimedOut = true
		return time.Time{}, os.ErrDeadlineExceeded
	}
	return c.writeDeadline, nil
}

func (c *appleTLSConn) markReadTimedOut() {
	c.deadlineAccess.Lock()
	c.readTimedOut = true
	c.deadlineAccess.Unlock()
}

func (c *appleTLSConn) markWriteTimedOut() {
	c.deadlineAccess.Lock()
	c.writeTimedOut = true
	c.deadlineAccess.Unlock()
}

func deadlineTimeoutMs(deadline time.Time) (int, bool) {
	if deadline.IsZero() {
		return -1, false
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0, true
	}
	return timeoutFromDuration(remaining), false
}

func (c *appleTLSConn) isClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

func (c *appleTLSConn) acquireClient() (*C.box_apple_tls_client_t, error) {
	c.ioAccess.Lock()
	defer c.ioAccess.Unlock()
	if c.isClosed() {
		return nil, net.ErrClosed
	}
	client := c.client
	if client == nil {
		return nil, net.ErrClosed
	}
	c.ioGroup.Add(1)
	return client, nil
}

func (c *appleTLSConn) releaseClient() {
	c.ioGroup.Done()
}

func (c *appleTLSConn) errorFromPointer(errorPtr *C.char) error {
	if errorPtr != nil {
		defer C.free(unsafe.Pointer(errorPtr))
		if c.isClosed() {
			return net.ErrClosed
		}
		return E.New(C.GoString(errorPtr))
	}
	return net.ErrClosed
}

type appleTLSReadWaiter struct {
	conn    *appleTLSConn
	options N.ReadWaitOptions
	results chan *C.box_apple_tls_read_result_t
}

var _ N.ReadWaiter = (*appleTLSReadWaiter)(nil)

func (w *appleTLSReadWaiter) InitializeReadWaiter(options N.ReadWaitOptions) (needCopy bool) {
	w.options = options
	if w.results == nil {
		w.results = make(chan *C.box_apple_tls_read_result_t, 1)
	}
	return false
}

func (w *appleTLSReadWaiter) WaitReadBuffer() (*buf.Buffer, error) {
	c := w.conn
	c.readAccess.Lock()
	defer c.readAccess.Unlock()
	if c.readEOF {
		return nil, io.EOF
	}
	maximumLen := readWaitFreeLen(w.options)
	if maximumLen <= 0 {
		return nil, io.ErrShortBuffer
	}
	timeoutMs, err := c.prepareReadTimeout()
	if err != nil {
		return nil, err
	}
	client, err := c.acquireClient()
	if err != nil {
		return nil, err
	}
	defer c.releaseClient()

	handle := cgo.NewHandle(w)
	defer handle.Delete()
	var errorPtr *C.char
	if !bool(C.box_apple_tls_client_read_async(client, C.size_t(maximumLen), C.uintptr_t(handle), &errorPtr)) {
		return nil, c.errorFromPointer(errorPtr)
	}

	var result *C.box_apple_tls_read_result_t
	if timeoutMs >= 0 {
		timer := time.NewTimer(time.Duration(timeoutMs) * time.Millisecond)
		defer timer.Stop()
		select {
		case result = <-w.results:
		case <-timer.C:
			C.box_apple_tls_client_cancel(client)
			result = <-w.results
			if result != nil {
				C.box_apple_tls_read_result_free(result)
			}
			c.markReadTimedOut()
			return nil, os.ErrDeadlineExceeded
		}
	} else {
		result = <-w.results
	}
	return c.readWaitResultToBuffer(result, w.options)
}

func (c *appleTLSConn) readWaitResultToBuffer(result *C.box_apple_tls_read_result_t, options N.ReadWaitOptions) (*buf.Buffer, error) {
	defer C.box_apple_tls_read_result_free(result)
	buffer := options.NewBuffer()
	if buffer.IsFull() {
		buffer.Release()
		return nil, io.ErrShortBuffer
	}
	startLen := buffer.Len()
	var eof C.bool
	var errorPtr *C.char
	n := C.box_apple_tls_read_result_copy(result, unsafe.Pointer(&buffer.FreeBytes()[0]), C.size_t(buffer.FreeLen()), &eof, &errorPtr)
	if n < 0 {
		buffer.Release()
		return nil, c.errorFromPointer(errorPtr)
	}
	if bool(eof) {
		c.readEOF = true
		if n == 0 {
			buffer.Release()
			return nil, io.EOF
		}
	}
	if n == 0 {
		buffer.Release()
		return nil, io.ErrNoProgress
	}
	buffer.Truncate(startLen + int(n))
	options.PostReturn(buffer)
	return buffer, nil
}

func readWaitFreeLen(options N.ReadWaitOptions) int {
	if options.IncreaseBuffer {
		return 65535 - options.FrontHeadroom - options.RearHeadroom
	}
	if options.MTU > 0 {
		return options.MTU
	}
	return buf.BufferSize - options.FrontHeadroom - options.RearHeadroom
}

//export box_apple_tls_read_callback
func box_apple_tls_read_callback(callbackHandle C.uintptr_t, result *C.box_apple_tls_read_result_t) {
	handle := cgo.Handle(callbackHandle)
	waiter, ok := handle.Value().(*appleTLSReadWaiter)
	if !ok {
		C.box_apple_tls_read_result_free(result)
		return
	}
	select {
	case waiter.results <- result:
	default:
		C.box_apple_tls_read_result_free(result)
	}
}

func (c *appleTLSConn) NetConn() net.Conn {
	return c.rawConn
}

func (c *appleTLSConn) HandshakeContext(ctx context.Context) error {
	return nil
}

func (c *appleTLSConn) ConnectionState() ConnectionState {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	return c.state
}

func parseAppleTLSState(state *C.box_apple_tls_state_t) (tls.ConnectionState, [][]byte, error) {
	rawCerts, peerCertificates, err := parseAppleCertChain(state.peer_cert_chain, state.peer_cert_chain_len)
	if err != nil {
		return tls.ConnectionState{}, nil, err
	}
	var negotiatedProtocol string
	if state.alpn != nil {
		negotiatedProtocol = C.GoString(state.alpn)
	}
	var serverName string
	if state.server_name != nil {
		serverName = C.GoString(state.server_name)
	}
	return tls.ConnectionState{
		Version:            uint16(state.version),
		HandshakeComplete:  true,
		CipherSuite:        uint16(state.cipher_suite),
		NegotiatedProtocol: negotiatedProtocol,
		ServerName:         serverName,
		PeerCertificates:   peerCertificates,
	}, rawCerts, nil
}

func parseAppleCertChain(chain *C.uint8_t, chainLen C.size_t) ([][]byte, []*x509.Certificate, error) {
	if chain == nil || chainLen == 0 {
		return nil, nil, nil
	}
	chainBytes := C.GoBytes(unsafe.Pointer(chain), C.int(chainLen))
	var (
		rawCerts         [][]byte
		peerCertificates []*x509.Certificate
	)
	for len(chainBytes) >= 4 {
		certificateLen := binary.BigEndian.Uint32(chainBytes[:4])
		chainBytes = chainBytes[4:]
		if len(chainBytes) < int(certificateLen) {
			return nil, nil, E.New("apple TLS: invalid certificate chain")
		}
		certificateData := append([]byte(nil), chainBytes[:certificateLen]...)
		certificate, err := x509.ParseCertificate(certificateData)
		if err != nil {
			return nil, nil, E.Cause(err, "parse peer certificate")
		}
		rawCerts = append(rawCerts, certificateData)
		peerCertificates = append(peerCertificates, certificate)
		chainBytes = chainBytes[certificateLen:]
	}
	if len(chainBytes) != 0 {
		return nil, nil, E.New("apple TLS: invalid certificate chain")
	}
	return rawCerts, peerCertificates, nil
}

func timeoutFromDuration(timeout time.Duration) int {
	if timeout <= 0 {
		return 0
	}
	timeoutMilliseconds := int64(timeout / time.Millisecond)
	if timeout%time.Millisecond != 0 {
		timeoutMilliseconds++
	}
	if timeoutMilliseconds > math.MaxInt32 {
		return math.MaxInt32
	}
	return int(timeoutMilliseconds)
}

func cStringOrNil(value string) *C.char {
	if value == "" {
		return nil
	}
	return C.CString(value)
}

func cFree(pointer *C.char) {
	if pointer != nil {
		C.free(unsafe.Pointer(pointer))
	}
}
