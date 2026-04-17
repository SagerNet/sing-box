//go:build windows && (amd64 || 386)

package tlsspoof

import (
	"encoding/hex"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newSpoofer(t *testing.T, conn net.Conn, method Method) rawSpoofer {
	t.Helper()
	s, err := newRawSpoofer(conn, method)
	require.NoError(t, err)
	return s
}

// Basic lifecycle: opening a spoofer against a live TCP conn installs
// the driver, spawns run(), then shuts down cleanly without ever
// injecting. Exercises the close path that cancels an in-flight Recv.
func TestIntegrationSpooferOpenClose(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := listener.Accept()
		accepted <- c
	}()
	client, err := net.Dial("tcp4", listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	server := <-accepted
	t.Cleanup(func() {
		if server != nil {
			server.Close()
		}
	})

	spoofer := newSpoofer(t, client, MethodWrongSequence)
	require.NoError(t, spoofer.Close())
}

// End-to-end: Conn.Write injects a fake ClientHello with a fresh SNI, then
// forwards the real ClientHello. With wrong-sequence, the fake lands before
// the connection's send-next sequence — the peer TCP stack treats it as
// already-received and only surfaces the real bytes to the echo server.
func TestIntegrationConnInjectsThenForwardsRealCH(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	serverReceived := make(chan []byte, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		got, _ := io.ReadAll(conn)
		serverReceived <- got
	}()

	client, err := net.Dial("tcp4", listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	wrapped, err := NewConn(client, MethodWrongSequence, "letsencrypt.org")
	require.NoError(t, err)

	payload, err := hex.DecodeString(realClientHello)
	require.NoError(t, err)

	n, err := wrapped.Write(payload)
	require.NoError(t, err)
	require.Equal(t, len(payload), n)
	_ = wrapped.Close()

	select {
	case got := <-serverReceived:
		require.Equal(t, payload, got,
			"server must receive real ClientHello unchanged (wrong-sequence fake must be dropped)")
	case <-time.After(5 * time.Second):
		t.Fatal("echo server did not receive real ClientHello within 5s")
	}
}

// Inject before any kernel payload: stages the fake, then Write flushes
// the real CH. Same terminal expectation as the Conn variant but via the
// raw spoofer primitive directly.
func TestIntegrationSpooferInjectThenWrite(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	serverReceived := make(chan []byte, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		got, _ := io.ReadAll(conn)
		serverReceived <- got
	}()

	client, err := net.Dial("tcp4", listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	spoofer := newSpoofer(t, client, MethodWrongSequence)
	t.Cleanup(func() { spoofer.Close() })

	fake, err := buildFakeClientHello("letsencrypt.org")
	require.NoError(t, err)
	require.NoError(t, spoofer.Inject(fake))

	payload, err := hex.DecodeString(realClientHello)
	require.NoError(t, err)
	n, err := client.Write(payload)
	require.NoError(t, err)
	require.Equal(t, len(payload), n)
	_ = client.Close()

	select {
	case got := <-serverReceived:
		require.Equal(t, payload, got)
	case <-time.After(5 * time.Second):
		t.Fatal("echo server did not receive real ClientHello within 5s")
	}
}
