//go:build windows && (amd64 || 386)

package tlsspoof

import (
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	M "github.com/sagernet/sing/common/metadata"

	"github.com/sagernet/sing-box/common/windivert"
	"github.com/sagernet/sing-tun/gtcpip/header"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// TestReadWindowsTCPMaxSeg_ReturnsNegotiatedValue runs without admin rights.
// It mirrors the POSIX TestReadTCPMaxSeg_ReturnsNegotiatedValue: confirms that
// TCP_MAXSEG is a readable getsockopt on a connected socket on this Windows
// build, so the Windows spoofer path does not have to fall back to the 1460
// default.
func TestReadWindowsTCPMaxSeg_ReturnsNegotiatedValue(t *testing.T) {
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

	mss, err := readWindowsTCPMaxSeg(client.(*net.TCPConn))
	require.NoError(t, err)
	require.Positive(t, mss, "TCP_MAXSEG must be readable post-handshake")
	require.Less(t, mss, 1<<16, "TCP_MAXSEG must be a sane IP-payload value")
}

// TestIntegrationSpooferStoresNegotiatedMSS installs the WinDivert-backed
// spoofer against a real connected TCP socket and asserts that construction
// populated the internal chunk budget from TCP_MAXSEG rather than the
// 1460 fallback. This is the Windows counterpart to the POSIX wire-order test
// — we cannot pcap the wire without npcap/pktmon in CI, so instead we verify
// the negotiated-MSS handoff that drives the per-chunk split the user needs.
func TestIntegrationSpooferStoresNegotiatedMSS(t *testing.T) {
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
	t.Cleanup(func() { spoofer.Close() })

	ws, ok := spoofer.(*windowsSpoofer)
	require.True(t, ok, "NewSpoofer must return *windowsSpoofer on this build")
	require.Positive(t, ws.mss, "spoofer must store a positive MSS")
	require.Less(t, ws.mss, 1<<16, "spoofer MSS must be a sane IP-payload value")
}

// TestIntegrationConnSplitsOversizedFakeOnWire is the Windows counterpart to
// the POSIX wire-order test. It opens a passive WinDivert sniffing handle
// alongside the spoofer, forces a small chunk budget so a standard
// ClientHello splits, and asserts the fake actually reaches the wire as N
// separate non-fragmented TCP segments with the expected per-chunk sequence
// numbers — not as one oversized packet the IP output path would fragment.
func TestIntegrationConnSplitsOversizedFakeOnWire(t *testing.T) {
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

	src := M.AddrPortFromNet(client.LocalAddr())
	dst := M.AddrPortFromNet(client.RemoteAddr())
	src = netip.AddrPortFrom(src.Addr().Unmap(), src.Port())
	dst = netip.AddrPortFrom(dst.Addr().Unmap(), dst.Port())

	// Passive sniff of the same 5-tuple the spoofer targets. Priority is
	// irrelevant for sniffing — the driver copies packets to us without
	// removing them from the stack or from the spoofer's divert handle.
	sniffFilter, err := windivert.OutboundTCP(src, dst)
	require.NoError(t, err)
	sniffH, err := windivert.Open(sniffFilter, windivert.LayerNetwork, 0, windivert.FlagSniff)
	require.NoError(t, err, "FlagSniff must open cleanly on this WinDivert build")
	t.Cleanup(func() { sniffH.Close() })

	// Drain the sniffer into a slice in the background so we don't miss
	// packets between emission and parse.
	var (
		access sync.Mutex
		events []tcpdumpEvent
	)
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, windivert.MTUMax)
		for {
			n, _, rerr := sniffH.Recv(buf)
			if rerr != nil {
				if errors.Is(rerr, windows.ERROR_OPERATION_ABORTED) ||
					errors.Is(rerr, windows.ERROR_NO_DATA) {
					return
				}
				return
			}
			ev, ok := parseSniffedTCP(buf[:n])
			if !ok {
				continue
			}
			access.Lock()
			events = append(events, ev)
			access.Unlock()
		}
	}()

	spoofer := newSpoofer(t, client, MethodWrongSequence)
	ws, ok := spoofer.(*windowsSpoofer)
	require.True(t, ok)
	const testMSS = 500
	ws.mss = testMSS
	wrapped := NewConn(client, spoofer, "letsencrypt.org")

	payload, err := hex.DecodeString(realClientHello)
	require.NoError(t, err)
	fakeBytes, err := rewriteSNI(payload, "letsencrypt.org")
	require.NoError(t, err)

	expectedFakeSegments := (len(fakeBytes) + testMSS - 1) / testMSS
	require.Greater(t, expectedFakeSegments, 1,
		"realClientHello + MSS must force splitting for this test to mean anything")

	n, err := wrapped.Write(payload)
	require.NoError(t, err)
	require.Equal(t, len(payload), n)

	select {
	case got := <-serverReceived:
		require.Equal(t, payload, got,
			"server must receive real ClientHello unchanged (wrong-sequence fake must be dropped)")
	case <-time.After(5 * time.Second):
		t.Fatal("echo server did not receive real ClientHello within 5s")
	}

	// Let the sniffer drain late arrivals, then shut it down.
	time.Sleep(200 * time.Millisecond)
	sniffH.Close()
	<-done

	// The spoofer captured snd_una during construction; the fake lives at
	// [sendNext-len(fake), sendNext). Group observed outbound payloads by
	// seq so that any WinDivert-internal duplication doesn't inflate counts.
	access.Lock()
	defer access.Unlock()

	// The Windows spoofer does not expose sendNext (it reads seq from the
	// captured real at run time), so classify by payload content: each fake
	// chunk is an exact slice of fakeBytes (rewriteSNI replaces only the
	// SNI, leaving the rest of the ClientHello bytes distinct from both the
	// unrewritten original and any kernel-generated framing).
	fakeSeqs := make(map[uint32]int)
	realSeqs := make(map[uint32]int)
	for _, ev := range events {
		if !ev.fromClient || ev.payloadLen == 0 {
			continue
		}
		if matchesFakeSlice(ev.rawPayload, fakeBytes) {
			fakeSeqs[ev.seq] = ev.payloadLen
		} else {
			realSeqs[ev.seq] = ev.payloadLen
		}
	}

	require.Equal(t, expectedFakeSegments, len(fakeSeqs),
		"fake must emit as %d separate raw TCP segments (fake=%v real=%v)",
		expectedFakeSegments, fakeSeqs, realSeqs)
	require.NotEmpty(t, realSeqs, "real ClientHello must also reach the wire")
	for seq, n := range fakeSeqs {
		require.LessOrEqual(t, n, testMSS,
			"fake segment seq=%d payload must be <=mss", seq)
	}
	var fakeBytesOnWire int
	for _, n := range fakeSeqs {
		fakeBytesOnWire += n
	}
	require.Equal(t, len(fakeBytes), fakeBytesOnWire,
		"sum of fake chunk payloads must equal the rewritten ClientHello size")
}

type tcpdumpEvent struct {
	seq        uint32
	payloadLen int
	rawPayload []byte
	fromClient bool
}

func parseSniffedTCP(pkt []byte) (tcpdumpEvent, bool) {
	if len(pkt) < header.IPv4MinimumSize+header.TCPMinimumSize {
		return tcpdumpEvent{}, false
	}
	ip := header.IPv4(pkt)
	if ip.Protocol() != uint8(header.TCPProtocolNumber) {
		return tcpdumpEvent{}, false
	}
	ihl := int(ip.HeaderLength())
	if ihl < header.IPv4MinimumSize || ihl+header.TCPMinimumSize > len(pkt) {
		return tcpdumpEvent{}, false
	}
	tcp := header.TCP(pkt[ihl:])
	tcpHdr := int(tcp.DataOffset())
	if tcpHdr < header.TCPMinimumSize || ihl+tcpHdr > len(pkt) {
		return tcpdumpEvent{}, false
	}
	total := int(ip.TotalLength())
	if total == 0 || total > len(pkt) {
		total = len(pkt)
	}
	payloadLen := total - ihl - tcpHdr
	var payload []byte
	if payloadLen > 0 {
		payload = append([]byte(nil), pkt[ihl+tcpHdr:ihl+tcpHdr+payloadLen]...)
	}
	return tcpdumpEvent{
		seq:        tcp.SequenceNumber(),
		payloadLen: payloadLen,
		rawPayload: payload,
		// Sniff returns outbound packets for OutboundTCP(src,dst), so by
		// definition everything matching is from-client.
		fromClient: true,
	}, true
}

// matchesFakeSlice reports whether `chunk` is a contiguous slice of `fake`.
// The Windows wire-order test uses this to recognise a fake segment without
// knowing the spoofer's snd_nxt reference.
func matchesFakeSlice(chunk, fake []byte) bool {
	if len(chunk) == 0 || len(chunk) > len(fake) {
		return false
	}
	for offset := 0; offset+len(chunk) <= len(fake); offset++ {
		match := true
		for i := 0; i < len(chunk); i++ {
			if chunk[i] != fake[offset+i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func newSpoofer(t *testing.T, conn net.Conn, method Method) Spoofer {
	t.Helper()
	spoofer, err := NewSpoofer(conn, method)
	require.NoError(t, err)
	return spoofer
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

// End-to-end: Conn.Write injects a fake ClientHello with a rewritten
// SNI, then forwards the real ClientHello. With wrong-sequence, the
// fake lands before the connection's send-next sequence — the peer TCP
// stack treats it as already-received and only surfaces the real bytes
// to the echo server.
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

	spoofer := newSpoofer(t, client, MethodWrongSequence)
	wrapped := NewConn(client, spoofer, "letsencrypt.org")

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
// Spoofer primitive directly.
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

	payload, err := hex.DecodeString(realClientHello)
	require.NoError(t, err)
	fake, err := rewriteSNI(payload, "letsencrypt.org")
	require.NoError(t, err)
	require.NoError(t, spoofer.Inject(fake))

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
