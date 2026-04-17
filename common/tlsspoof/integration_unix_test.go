//go:build linux || darwin

package tlsspoof

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestReadTCPMaxSeg_ReturnsNegotiatedValue runs without root. It verifies that
// MSS discovery from a real connected TCP socket returns a positive value
// matching a plausible loopback/Ethernet MSS, so the raw-injection path no
// longer has to fall back to the hardcoded 1460/1440 default on machines where
// the kernel ABI keeps TCP_MAXSEG readable after handshake.
func TestReadTCPMaxSeg_ReturnsNegotiatedValue(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
		close(accepted)
	}()

	client, err := net.Dial("tcp4", listener.Addr().String())
	require.NoError(t, err)
	defer client.Close()

	server := <-accepted
	require.NotNil(t, server)
	defer server.Close()

	mss, err := readTCPMaxSeg(client.(*net.TCPConn))
	require.NoError(t, err)
	require.Positive(t, mss, "TCP_MAXSEG must be readable post-handshake")
	require.Less(t, mss, 1<<16, "TCP_MAXSEG must be a sane IP-payload value")
}

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

func TestIntegrationSpoofer_IPv6_WrongChecksum(t *testing.T) {
	requireRoot(t)
	client, serverPort := dialLocalEchoServerIPv6(t)
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

func TestIntegrationSpoofer_IPv6_WrongSequence(t *testing.T) {
	requireRoot(t)
	client, serverPort := dialLocalEchoServerIPv6(t)
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
	runInjectsThenForwardsRealCH(t, "tcp4", "127.0.0.1:0")
}

func TestIntegrationConn_IPv6_InjectsThenForwardsRealCH(t *testing.T) {
	requireRoot(t)
	runInjectsThenForwardsRealCH(t, "tcp6", "[::1]:0")
}

// TestIntegrationConn_FakeAndRealHaveDistinctSNIs asserts that the on-wire fake
// packet carries the fake SNI (letsencrypt.org) AND the real packet still
// carries the original SNI (github.com). If the rewriter regresses to returning
// the original bytes unchanged, the fake-SNI needle will be missing.
func TestIntegrationConn_FakeAndRealHaveDistinctSNIs(t *testing.T) {
	requireRoot(t)
	runFakeAndRealHaveDistinctSNIs(t, "tcp4", "127.0.0.1:0", "letsencrypt.org")
}

func TestIntegrationConn_IPv6_FakeAndRealHaveDistinctSNIs(t *testing.T) {
	requireRoot(t)
	runFakeAndRealHaveDistinctSNIs(t, "tcp6", "[::1]:0", "letsencrypt.org")
}

// TestIntegrationConn_FakeSNISameLength exercises the delta=0 rewrite path.
// "example.co" is 10 bytes, the same length as the original "github.com" SNI
// in realClientHello, so the rewritten record is the same length as the input.
// A rewriter that accidentally returns the input unchanged would leave
// "github.com" in the fake and the "example.co" needle would not appear.
func TestIntegrationConn_FakeSNISameLength(t *testing.T) {
	requireRoot(t)
	runFakeAndRealHaveDistinctSNIs(t, "tcp4", "127.0.0.1:0", "example.co")
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

	spoofer, err := NewSpoofer(client, MethodWrongSequence)
	require.NoError(t, err)
	wrapped := NewConn(client, spoofer, fakeSNI)

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

// TestIntegrationConn_SplitsOversizedFakeOnWire forces the spoofer to chunk
// the fake at a small MSS and verifies that the raw-socket path actually emits
// every chunk as a separate non-fragmented TCP segment toward the peer. It is
// the only test that observes what reaches the wire rather than what the
// builders produce — the one the user asked for.
func TestIntegrationConn_SplitsOversizedFakeOnWire(t *testing.T) {
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
	// Small, prime-ish budget so the chunk count is unambiguous.
	const testMSS = 500
	require.True(t, forceMSS(spoofer, testMSS), "forceMSS must reach the platform spoofer")
	sendNext, ok := spooferSendNext(spoofer)
	require.True(t, ok, "spooferSendNext must reach the platform spoofer")
	wrapped := NewConn(client, spoofer, "letsencrypt.org")

	payload, err := hex.DecodeString(realClientHello)
	require.NoError(t, err)
	fakeBytes, err := rewriteSNI(payload, "letsencrypt.org")
	require.NoError(t, err)

	// The fake's chunks live in [sendNext-len(fake), sendNext); the real
	// starts at sendNext. Both assertions use the absolute wire seq so the
	// uint32 wraparound cancels on subtraction.
	expectedFakeSegments := (len(fakeBytes) + testMSS - 1) / testMSS
	expectedFirstFakeSeq := sendNext - uint32(len(fakeBytes))

	events := tcpdumpSequenceObserver(t, loopbackInterface, serverPort, func() {
		n, werr := wrapped.Write(payload)
		require.NoError(t, werr)
		require.Equal(t, len(payload), n)
	}, 3*time.Second)

	// Count outbound TCP segments in the capture: group by seq so a
	// retransmitted ACK does not inflate counts, and only consider segments
	// with payload >0 (SYN/SYN-ACK/pure-ACKs carry no TLS data).
	var (
		fakeSeqs = make(map[uint32]int)
		realSeqs = make(map[uint32]int)
	)
	for _, ev := range events {
		if !ev.fromClient || ev.payloadLen == 0 {
			continue
		}
		// wrong-sequence fake lives in [initialSeq-len, initialSeq); real
		// starts at initialSeq. Wraparound is fine under uint32 arithmetic.
		if ev.seq-expectedFirstFakeSeq < uint32(len(fakeBytes)) {
			fakeSeqs[ev.seq] += ev.payloadLen
		} else {
			realSeqs[ev.seq] += ev.payloadLen
		}
	}

	require.Equal(t, expectedFakeSegments, len(fakeSeqs),
		"fake must emit as %d separate raw TCP segments (got seqs=%v)", expectedFakeSegments, fakeSeqs)
	require.NotEmpty(t, realSeqs, "real ClientHello must also reach the wire")

	// Every fake chunk must be <=MSS at the TCP layer, i.e. no IP
	// fragmentation from our side. On loopback fragmentation can't happen
	// anyway — but the per-chunk size invariant is the end-to-end guarantee
	// the fix promises.
	for seq, total := range fakeSeqs {
		require.LessOrEqual(t, total, testMSS,
			"fake segment seq=%d payload must be <=mss", seq)
	}
	// The full fake payload must be covered by the sum of chunk payloads.
	var fakeBytesOnWire int
	for _, n := range fakeSeqs {
		fakeBytesOnWire += n
	}
	require.Equal(t, len(fakeBytes), fakeBytesOnWire,
		"sum of fake chunk payloads must equal the rewritten ClientHello size")

	_ = wrapped.Close()
	select {
	case got := <-serverReceived:
		require.Equal(t, payload, got,
			"server must receive real ClientHello unchanged (wrong-sequence fake is dropped)")
	case <-time.After(2 * time.Second):
		t.Fatal("echo server did not receive real ClientHello")
	}
}

// tcpdumpEvent captures what we can parse out of a single tcpdump line for
// wire-order assertions.
type tcpdumpEvent struct {
	seq        uint32
	payloadLen int
	fromClient bool
}

// tcpdumpSequenceObserver runs tcpdump against the loopback interface and
// collects every TCP segment visible during do(). Unlike tcpdumpObserverMulti
// it parses absolute sequence numbers (-S) and payload lengths so callers can
// reason about splitting, not just about textual payload needles.
func tcpdumpSequenceObserver(t *testing.T, iface string, port uint16, do func(), wait time.Duration) []tcpdumpEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tcpdump", "-i", iface, "-n", "-l", "-S",
		fmt.Sprintf("tcp and port %d", port))
	cmd.Cancel = func() error {
		return cmd.Process.Signal(os.Interrupt)
	}
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	})

	ready := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "listening on") {
				close(ready)
				io.Copy(io.Discard, stderr)
				return
			}
		}
	}()

	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("tcpdump did not attach within 2s")
	}

	var (
		access     sync.Mutex
		events     []tcpdumpEvent
		readerDone = make(chan struct{})
	)
	go func() {
		defer close(readerDone)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			ev, ok := parseTCPDumpLine(line, port)
			if !ok {
				continue
			}
			access.Lock()
			events = append(events, ev)
			access.Unlock()
		}
	}()

	do()

	time.Sleep(200 * time.Millisecond)
	_ = cmd.Process.Signal(os.Interrupt)
	<-readerDone
	access.Lock()
	defer access.Unlock()
	out := make([]tcpdumpEvent, len(events))
	copy(out, events)
	return out
}

// Example tcpdump -S -n line:
//
//	20:38:48.123456 IP 127.0.0.1.53412 > 127.0.0.1.443: Flags [P.], seq 4294965789:4294966289, ack 1, win 65535, length 500
var tcpdumpLineRE = regexp.MustCompile(
	`IP6?\s+\S+\.(\d+)\s+>\s+\S+\.(\d+):\s+Flags \[[^\]]+\],\s+seq\s+(\d+)(?::\d+)?,.*?length\s+(\d+)`,
)

func parseTCPDumpLine(line string, targetPort uint16) (tcpdumpEvent, bool) {
	m := tcpdumpLineRE.FindStringSubmatch(line)
	if m == nil {
		return tcpdumpEvent{}, false
	}
	srcPort, err := strconv.ParseUint(m[1], 10, 16)
	if err != nil {
		return tcpdumpEvent{}, false
	}
	dstPort, err := strconv.ParseUint(m[2], 10, 16)
	if err != nil {
		return tcpdumpEvent{}, false
	}
	seq, err := strconv.ParseUint(m[3], 10, 32)
	if err != nil {
		return tcpdumpEvent{}, false
	}
	length, err := strconv.Atoi(m[4])
	if err != nil {
		return tcpdumpEvent{}, false
	}
	return tcpdumpEvent{
		seq:        uint32(seq),
		payloadLen: length,
		fromClient: uint16(dstPort) == targetPort && uint16(srcPort) != targetPort,
	}, true
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
