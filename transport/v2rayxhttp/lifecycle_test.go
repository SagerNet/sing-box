package v2rayxhttp

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/stretchr/testify/require"
)

func TestTransportLongLivedConnections(t *testing.T) {
	for _, mode := range []string{"stream-one", "stream-up", "packet-up"} {
		t.Run(mode, func(t *testing.T) {
			client, _ := newLifecycleTransport(t, mode, nil)
			connection, err := client.DialContext(context.Background())
			require.NoError(t, err)
			defer connection.Close()

			for index := range 32 {
				payload := bytes.Repeat([]byte{byte(index)}, 2048)
				_, err = connection.Write(payload)
				require.NoError(t, err)
				response := make([]byte, len(payload))
				_, err = io.ReadFull(connection, response)
				require.NoError(t, err)
				require.Equal(t, payload, response)
			}
		})
	}
}

func TestServerSessionReordersDelayedPackets(t *testing.T) {
	session := newServerSession(4)
	t.Cleanup(session.close)

	// The second packet arrives first. It must not be exposed until the missing
	// first packet arrives, then both packets must be delivered in sequence.
	require.True(t, session.push(packet{sequence: 1, payload: []byte("B")}))
	readResult := make(chan []byte, 1)
	readError := make(chan error, 1)
	go func() {
		payload := make([]byte, 2)
		_, err := io.ReadFull(session.reader, payload)
		if err != nil {
			readError <- err
			return
		}
		readResult <- payload
	}()

	select {
	case payload := <-readResult:
		t.Fatalf("received packet data before the sequence gap was filled: %q", payload)
	case err := <-readError:
		t.Fatalf("read failed before the sequence gap was filled: %v", err)
	case <-time.After(75 * time.Millisecond):
	}

	require.True(t, session.push(packet{sequence: 0, payload: []byte("A")}))
	select {
	case payload := <-readResult:
		require.Equal(t, []byte("AB"), payload)
	case err := <-readError:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reordered packets")
	}
}

func TestPacketUpReordersDelayedHTTPRequests(t *testing.T) {
	options := option.V2RayXHTTPOptions{Path: "/xhttp", Mode: "packet-up"}
	server, err := NewServer(context.Background(), logger.NOP(), options, nil, echoHandler{})
	require.NoError(t, err)
	listener := newTrackingListener(t)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
	})

	httpClient := &http.Client{Transport: &http.Transport{}}
	t.Cleanup(func() { httpClient.CloseIdleConnections() })
	endpoint := "http://" + listener.Addr().String() + options.Path
	sendPacket := func(sequence uint64, payload []byte) {
		request, requestErr := http.NewRequest(http.MethodPost, endpoint, nil)
		require.NoError(t, requestErr)
		server.config.fillPacketRequest(request, "delayed", sequence, payload)
		response, requestErr := httpClient.Do(request)
		require.NoError(t, requestErr)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.NoError(t, response.Body.Close())
	}

	// Simulate a delayed (or retransmitted) first request: packet 1 is accepted
	// but cannot cross the session pipe until packet 0 arrives.
	sendPacket(1, []byte("B"))
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	require.NoError(t, err)
	server.config.fillStreamRequest(request, "delayed")
	response, err := httpClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)

	readResult := make(chan []byte, 1)
	readError := make(chan error, 1)
	go func() {
		payload := make([]byte, 2)
		_, readErr := io.ReadFull(response.Body, payload)
		if readErr != nil {
			readError <- readErr
			return
		}
		readResult <- payload
	}()
	select {
	case payload := <-readResult:
		t.Fatalf("received HTTP packet data before the sequence gap was filled: %q", payload)
	case readErr := <-readError:
		t.Fatalf("HTTP read failed before the sequence gap was filled: %v", readErr)
	case <-time.After(75 * time.Millisecond):
	}

	sendPacket(0, []byte("A"))
	select {
	case payload := <-readResult:
		require.Equal(t, []byte("AB"), payload)
	case readErr := <-readError:
		require.NoError(t, readErr)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reordered HTTP packets")
	}
}

func TestServerSessionExpiryReclaimsOrphan(t *testing.T) {
	server, err := NewServer(context.Background(), logger.NOP(), option.V2RayXHTTPOptions{Path: "/xhttp", Mode: "packet-up"}, nil, echoHandler{})
	require.NoError(t, err)
	server.sessionTimeout = 50 * time.Millisecond
	t.Cleanup(func() { _ = server.Close() })

	session := server.session("orphan")
	waitForLifecycle(t, time.Second, func() bool {
		_, exists := server.sessions.Load("orphan")
		return !exists
	})
	select {
	case <-session.closed:
	default:
		t.Fatal("expired session was removed without closing its streams")
	}
}

func TestTransportResourceCleanup(t *testing.T) {
	beforeGoroutines := runtime.NumGoroutine()
	for _, mode := range []string{"stream-one", "stream-up", "packet-up"} {
		t.Run(mode, func(t *testing.T) {
			listener := newTrackingListener(t)
			client, server := newLifecycleTransport(t, mode, listener)
			for index := 0; index < 12; index++ {
				connection, err := client.DialContext(context.Background())
				require.NoError(t, err)
				payload := []byte{byte(index), 'x', 'h', 't', 't', 'p'}
				_, err = connection.Write(payload)
				require.NoError(t, err)
				response := make([]byte, len(payload))
				_, err = io.ReadFull(connection, response)
				require.NoError(t, err)
				require.Equal(t, payload, response)
				require.NoError(t, connection.Close())
			}
			_ = client.Close()
			_ = server.Close()
			_ = listener.Close()
			waitForLifecycle(t, 2*time.Second, func() bool { return listener.active.Load() == 0 })
		})
	}

	runtime.GC()
	runtime.GC()
	// The package test process itself retains a few HTTP and test-runner
	// goroutines. A small bounded delta catches leaked request/session workers
	// without making the check dependent on their exact implementation.
	waitForLifecycle(t, 2*time.Second, func() bool {
		return runtime.NumGoroutine() <= beforeGoroutines+12
	})
}

func newLifecycleTransport(t *testing.T, mode string, listener *trackingListener) (*Client, *Server) {
	t.Helper()
	options := option.V2RayXHTTPOptions{
		Path:                 "/xhttp",
		Mode:                 mode,
		SCMaxEachPostBytes:   option.V2RayXHTTPRange{From: 2048, To: 2048},
		SCMinPostsIntervalMS: option.V2RayXHTTPRange{From: 1, To: 1},
		SCMaxBufferedPosts:   8,
	}
	server, err := NewServer(context.Background(), logger.NOP(), options, nil, echoHandler{})
	require.NoError(t, err)
	if listener == nil {
		listener = newTrackingListener(t)
	}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
	})

	dialer, err := dialer.NewDefault(context.Background(), option.DialerOptions{})
	require.NoError(t, err)
	client, err := NewClient(context.Background(), dialer, M.SocksaddrFromNet(listener.Addr()), options, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client, server
}

func waitForLifecycle(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for XHTTP lifecycle cleanup")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type trackingListener struct {
	net.Listener
	active atomic.Int64
}

func newTrackingListener(t *testing.T) *trackingListener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return &trackingListener{Listener: listener}
}

func (l *trackingListener) Accept() (net.Conn, error) {
	connection, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	l.active.Add(1)
	return &trackingConn{Conn: connection, onClose: func() { l.active.Add(-1) }}, nil
}

type trackingConn struct {
	net.Conn
	onClose func()
	once    sync.Once
}

func (c *trackingConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.onClose)
	return err
}
