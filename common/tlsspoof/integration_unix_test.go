//go:build linux || darwin

package tlsspoof

import (
	"encoding/hex"
	"io"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIntegrationSpoofer(t *testing.T) {
	requireRoot(t)
	methods := []struct {
		name   string
		method Method
	}{
		{"WrongChecksum", MethodWrongChecksum},
		{"WrongSequence", MethodWrongSequence},
		{"WrongAcknowledgment", MethodWrongAcknowledgment},
		{"WrongMD5Sig", MethodWrongMD5Sig},
		{"WrongTimestamp", MethodWrongTimestamp},
	}
	families := []struct {
		name string
		dial func(*testing.T) (net.Conn, uint16)
	}{
		{"IPv4", dialLocalEchoServer},
		{"IPv6", dialLocalEchoServerIPv6},
	}
	for _, family := range families {
		for _, tc := range methods {
			t.Run(family.name+"/"+tc.name, func(t *testing.T) {
				if tc.method == MethodWrongTimestamp && runtime.GOOS == "darwin" {
					t.Skip("wrong-timestamp is not supported on macOS")
				}
				client, serverPort := family.dial(t)
				spoofer, err := newRawSpoofer(client, tc.method)
				require.NoError(t, err)
				defer spoofer.Close()

				fake, err := buildFakeClientHello("letsencrypt.org")
				require.NoError(t, err)

				captured := tcpdumpObserver(t, loopbackInterface, serverPort, "letsencrypt.org", func() {
					require.NoError(t, spoofer.Inject(fake))
				}, 3*time.Second)
				require.True(t, captured, "injected fake ClientHello must be observable on loopback")
			})
		}
	}
}

// Loopback bypasses TCP checksum validation, so wrong-sequence is used instead.
func TestIntegrationConn_InjectsThenForwardsRealCH(t *testing.T) {
	requireRoot(t)
	runInjectsThenForwardsRealCH(t, "tcp4", "127.0.0.1:0")
}

func TestIntegrationConn_IPv6_InjectsThenForwardsRealCH(t *testing.T) {
	requireRoot(t)
	runInjectsThenForwardsRealCH(t, "tcp6", "[::1]:0")
}

// TestIntegrationConn_FakeAndRealHaveDistinctSNIs asserts that the on-wire fake
// packet carries the fake SNI (letsencrypt.org) AND the real packet still
// carries the original SNI (github.com). If the builder regresses to producing
// empty or mismatched bytes, the fake-SNI needle will be missing.
func TestIntegrationConn_FakeAndRealHaveDistinctSNIs(t *testing.T) {
	requireRoot(t)
	runFakeAndRealHaveDistinctSNIs(t, "tcp4", "127.0.0.1:0", "letsencrypt.org")
}

func TestIntegrationConn_IPv6_FakeAndRealHaveDistinctSNIs(t *testing.T) {
	requireRoot(t)
	runFakeAndRealHaveDistinctSNIs(t, "tcp6", "[::1]:0", "letsencrypt.org")
}

func runFakeAndRealHaveDistinctSNIs(t *testing.T, network, address, fakeSNI string) {
	t.Helper()
	const originalSNI = "github.com"
	require.NotEqual(t, originalSNI, fakeSNI)

	listener, err := net.Listen(network, address)
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
	client, err := net.Dial(network, addr.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Close()
		listener.Close()
	})

	wrapped, err := NewConn(client, MethodWrongSequence, fakeSNI)
	require.NoError(t, err)

	payload, err := hex.DecodeString(realClientHello)
	require.NoError(t, err)

	seen := tcpdumpObserverMulti(t, loopbackInterface, serverPort,
		[]string{originalSNI, fakeSNI}, func() {
			n, err := wrapped.Write(payload)
			require.NoError(t, err)
			require.Equal(t, len(payload), n)
		}, 3*time.Second)
	require.True(t, seen[originalSNI],
		"real ClientHello must carry original SNI %q on the wire", originalSNI)
	require.True(t, seen[fakeSNI],
		"fake ClientHello must carry fake SNI %q on the wire", fakeSNI)

	_ = wrapped.Close()
	select {
	case got := <-serverReceived:
		require.Equal(t, payload, got,
			"server must receive real ClientHello unchanged (wrong-sequence fake must be dropped)")
	case <-time.After(2 * time.Second):
		t.Fatal("echo server did not receive real ClientHello")
	}
}

func runInjectsThenForwardsRealCH(t *testing.T, network, address string) {
	t.Helper()
	listener, err := net.Listen(network, address)
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
	client, err := net.Dial(network, addr.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Close()
		listener.Close()
	})

	wrapped, err := NewConn(client, MethodWrongSequence, "letsencrypt.org")
	require.NoError(t, err)

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
