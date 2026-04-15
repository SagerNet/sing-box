//go:build linux || darwin

package tlsspoof

import (
	"encoding/hex"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIntegrationSpoofer_WrongChecksum(t *testing.T) {
	requireRoot(t)
	client, serverPort := dialLocalEchoServer(t)
	spoofer, err := NewSpoofer(client, MethodWrongChecksum)
	require.NoError(t, err)
	defer spoofer.Close()

	payload, err := hex.DecodeString(realClientHello)
	require.NoError(t, err)
	fake, err := rewriteSNI(payload, "letsencrypt.org")
	require.NoError(t, err)

	captured := tcpdumpObserver(t, loopbackInterface, serverPort, "letsencrypt.org", func() {
		require.NoError(t, spoofer.Inject(fake))
	}, 3*time.Second)
	require.True(t, captured, "injected fake ClientHello must be observable on loopback")
}

func TestIntegrationSpoofer_WrongSequence(t *testing.T) {
	requireRoot(t)
	client, serverPort := dialLocalEchoServer(t)
	spoofer, err := NewSpoofer(client, MethodWrongSequence)
	require.NoError(t, err)
	defer spoofer.Close()

	payload, err := hex.DecodeString(realClientHello)
	require.NoError(t, err)
	fake, err := rewriteSNI(payload, "letsencrypt.org")
	require.NoError(t, err)

	captured := tcpdumpObserver(t, loopbackInterface, serverPort, "letsencrypt.org", func() {
		require.NoError(t, spoofer.Inject(fake))
	}, 3*time.Second)
	require.True(t, captured, "injected fake ClientHello must be observable on loopback")
}

// Loopback bypasses TCP checksum validation, so wrong-sequence is used instead.
func TestIntegrationConn_InjectsThenForwardsRealCH(t *testing.T) {
	requireRoot(t)

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)

	serverReceived := make(chan []byte, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		got, _ := io.ReadAll(conn)
		serverReceived <- got
	}()

	addr := listener.Addr().(*net.TCPAddr)
	serverPort := uint16(addr.Port)
	client, err := net.Dial("tcp4", addr.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Close()
		listener.Close()
	})

	spoofer, err := NewSpoofer(client, MethodWrongSequence)
	require.NoError(t, err)
	wrapped := NewConn(client, spoofer, "letsencrypt.org")

	payload, err := hex.DecodeString(realClientHello)
	require.NoError(t, err)

	captured := tcpdumpObserver(t, loopbackInterface, serverPort, "letsencrypt.org", func() {
		n, err := wrapped.Write(payload)
		require.NoError(t, err)
		require.Equal(t, len(payload), n)
	}, 3*time.Second)
	require.True(t, captured, "fake ClientHello with letsencrypt.org SNI must be on the wire")

	_ = wrapped.Close()
	select {
	case got := <-serverReceived:
		require.Equal(t, payload, got, "server must receive real ClientHello unchanged (wrong-sequence fake must be dropped)")
	case <-time.After(2 * time.Second):
		t.Fatal("echo server did not receive real ClientHello")
	}
}
