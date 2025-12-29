package route

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

type testLogger struct {
	logs []string
}

func (l *testLogger) Trace(args ...any) {}
func (l *testLogger) Debug(args ...any) {}
func (l *testLogger) Info(args ...any) {
	msg := ""
	for _, arg := range args {
		msg += arg.(string)
	}
	l.logs = append(l.logs, msg)
}
func (l *testLogger) Warn(args ...any)                              {}
func (l *testLogger) Error(args ...any)                             {}
func (l *testLogger) Fatal(args ...any)                             {}
func (l *testLogger) Panic(args ...any)                             {}
func (l *testLogger) TraceContext(ctx context.Context, args ...any) {}
func (l *testLogger) DebugContext(ctx context.Context, args ...any) {}
func (l *testLogger) InfoContext(ctx context.Context, args ...any)  {}
func (l *testLogger) WarnContext(ctx context.Context, args ...any)  {}
func (l *testLogger) ErrorContext(ctx context.Context, args ...any) {}
func (l *testLogger) FatalContext(ctx context.Context, args ...any) {}
func (l *testLogger) PanicContext(ctx context.Context, args ...any) {}

type mockConn struct {
	net.Conn
	readData  []byte
	writeData []byte
	readPos   int
}

func newMockConn(data []byte) *mockConn {
	return &mockConn{
		readData: data,
	}
}

func (c *mockConn) Read(b []byte) (n int, err error) {
	if c.readPos >= len(c.readData) {
		return 0, io.EOF
	}
	n = copy(b, c.readData[c.readPos:])
	c.readPos += n
	return n, nil
}

func (c *mockConn) Write(b []byte) (n int, err error) {
	c.writeData = append(c.writeData, b...)
	return len(b), nil
}

func (c *mockConn) Close() error {
	return nil
}

func (c *mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
}

func (c *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5678}
}

func (c *mockConn) SetDeadline(t time.Time) error      { return nil }
func (c *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *mockConn) SetWriteDeadline(t time.Time) error { return nil }

type mockPacketConn struct {
	N.PacketConn
	packets [][]byte
}

func (c *mockPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	if len(c.packets) == 0 {
		return M.Socksaddr{}, io.EOF
	}
	packet := c.packets[0]
	c.packets = c.packets[1:]
	_, err = buffer.Write(packet)
	return M.Socksaddr{}, err
}

func (c *mockPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	data := buffer.Bytes()
	c.packets = append(c.packets, data)
	return nil
}

func (c *mockPacketConn) Close() error {
	return nil
}

func (c *mockPacketConn) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
}

func (c *mockPacketConn) SetDeadline(t time.Time) error      { return nil }
func (c *mockPacketConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *mockPacketConn) SetWriteDeadline(t time.Time) error { return nil }

// newTestActivityTracker creates an ActivityTracker with a custom exit function and check interval for testing
func newTestActivityTracker(logger *testLogger, timeout time.Duration, checkInterval time.Duration, exitFunc func()) *ActivityTracker {
	tracker := NewActivityTracker(logger, timeout)
	tracker.checkInterval = checkInterval
	tracker.exitFunc = exitFunc
	return tracker
}

func TestActivityTracker_UpdateActivity(t *testing.T) {
	logger := &testLogger{}
	tracker := NewActivityTracker(logger, 5*time.Second)

	initialTime := time.Unix(0, tracker.lastActivity.Load())
	time.Sleep(10 * time.Millisecond)

	// Simulate activity
	tracker.updateActivity(100)

	updatedTime := time.Unix(0, tracker.lastActivity.Load())
	require.True(t, updatedTime.After(initialTime), "Activity timestamp should be updated")
}

func TestActivityTracker_UpdateActivityZeroBytes(t *testing.T) {
	logger := &testLogger{}
	tracker := NewActivityTracker(logger, 5*time.Second)

	initialTime := time.Unix(0, tracker.lastActivity.Load())
	time.Sleep(10 * time.Millisecond)

	// Simulate zero-byte activity (should not update)
	tracker.updateActivity(0)

	updatedTime := time.Unix(0, tracker.lastActivity.Load())
	require.Equal(t, initialTime.UnixNano(), updatedTime.UnixNano(), "Activity timestamp should not update for zero bytes")
}

func TestActivityTracker_RoutedConnection(t *testing.T) {
	logger := &testLogger{}
	tracker := NewActivityTracker(logger, 5*time.Second)

	mockConn := newMockConn([]byte("test data"))
	ctx := context.Background()
	metadata := adapter.InboundContext{}

	wrappedConn := tracker.RoutedConnection(ctx, mockConn, metadata, nil, nil)
	require.NotNil(t, wrappedConn)

	initialTime := time.Unix(0, tracker.lastActivity.Load())
	time.Sleep(10 * time.Millisecond)

	// Reading from wrapped connection should update activity
	buf := make([]byte, 1024)
	n, err := wrappedConn.Read(buf)
	require.NoError(t, err)
	require.Greater(t, n, 0)

	updatedTime := time.Unix(0, tracker.lastActivity.Load())
	require.True(t, updatedTime.After(initialTime), "Activity should be updated after read")
}

func TestActivityTracker_RoutedConnectionWrite(t *testing.T) {
	logger := &testLogger{}
	tracker := NewActivityTracker(logger, 5*time.Second)

	mockConn := newMockConn([]byte{})
	ctx := context.Background()
	metadata := adapter.InboundContext{}

	wrappedConn := tracker.RoutedConnection(ctx, mockConn, metadata, nil, nil)
	require.NotNil(t, wrappedConn)

	initialTime := time.Unix(0, tracker.lastActivity.Load())
	time.Sleep(10 * time.Millisecond)

	// Writing to wrapped connection should update activity
	n, err := wrappedConn.Write([]byte("test write"))
	require.NoError(t, err)
	require.Greater(t, n, 0)

	updatedTime := time.Unix(0, tracker.lastActivity.Load())
	require.True(t, updatedTime.After(initialTime), "Activity should be updated after write")
}

func TestActivityTracker_RoutedPacketConnection(t *testing.T) {
	logger := &testLogger{}
	tracker := NewActivityTracker(logger, 5*time.Second)

	mockPacket := &mockPacketConn{packets: [][]byte{[]byte("test packet")}}
	ctx := context.Background()
	metadata := adapter.InboundContext{}

	wrappedConn := tracker.RoutedPacketConnection(ctx, mockPacket, metadata, nil, nil)
	require.NotNil(t, wrappedConn)

	initialTime := time.Unix(0, tracker.lastActivity.Load())
	time.Sleep(10 * time.Millisecond)

	// Reading packet should update activity
	buffer := buf.New()
	defer buffer.Release()
	_, err := wrappedConn.ReadPacket(buffer)
	require.NoError(t, err)

	updatedTime := time.Unix(0, tracker.lastActivity.Load())
	require.True(t, updatedTime.After(initialTime), "Activity should be updated after packet read")
}

func TestActivityTracker_Lifecycle(t *testing.T) {
	logger := &testLogger{}
	tracker := NewActivityTracker(logger, 5*time.Second)

	// Test Start
	err := tracker.Start()
	require.NoError(t, err)

	// Verify monitor is running by checking that done channel is not closed
	select {
	case <-tracker.done:
		t.Fatal("done channel should not be closed after Start")
	default:
		// Expected
	}

	// Test Close
	err = tracker.Close()
	require.NoError(t, err)

	// Verify done channel is closed
	select {
	case <-tracker.done:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("done channel should be closed after Close")
	}

	// Test double close (should not error)
	err = tracker.Close()
	require.NoError(t, err)
}

func TestActivityTracker_TimeoutMonitoring(t *testing.T) {
	logger := &testLogger{}
	timeout := 50 * time.Millisecond

	// Channel to track if exit was called
	exitCalled := make(chan struct{})
	exitFunc := func() {
		close(exitCalled)
	}

	checkInterval := 20 * time.Millisecond // Fast interval for testing

	tracker := newTestActivityTracker(logger, timeout, checkInterval, exitFunc)

	// Set lastActivity to past so timeout will trigger immediately
	pastTime := time.Now().Add(-100 * time.Millisecond)
	tracker.lastActivity.Store(pastTime.UnixNano())

	// Start monitoring
	err := tracker.Start()
	require.NoError(t, err)
	defer tracker.Close()

	// Wait for exit to be called (should happen quickly with our fast check interval)
	select {
	case <-exitCalled:
		// Exit was called as expected
		require.Contains(t, logger.logs[0], "idle timeout reached after")
		require.Contains(t, logger.logs[0], "of inactivity, exiting")
	case <-time.After(200 * time.Millisecond):
		// Exit should have been called by now
		t.Fatal("Exit function was not called within expected time")
	}
}
