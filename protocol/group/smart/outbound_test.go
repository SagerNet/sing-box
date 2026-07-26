package smart

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/interrupt"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/protocol/group/smart/engine"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type mockOutbound struct {
	outbound.Adapter
	fail      bool
	handshake bool
	delay     time.Duration
	calls     *atomic.Int32
}

func (m *mockOutbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if m.calls != nil {
		m.calls.Add(1)
	}
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.delay):
		}
	}
	if m.fail {
		return nil, errors.New("dial failed")
	}
	c1, c2 := net.Pipe()
	if m.handshake {
		go func() {
			defer c2.Close()
			buffer := make([]byte, 4)
			_, _ = c2.Read(buffer)
			_, _ = c2.Write([]byte("ok"))
		}()
		return &handshakeMockConn{Conn: c1}, nil
	}
	go c2.Close()
	return c1, nil
}

func (m *mockOutbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("udp not implemented")
}

type handshakeMockConn struct {
	net.Conn
}

func (*handshakeMockConn) NeedHandshake() bool {
	return true
}

type reorderedWriteConn struct {
	net.Conn
	firstStarted chan struct{}
	releaseFirst chan struct{}
	calls        atomic.Int32
}

func (c *reorderedWriteConn) Write(buffer []byte) (int, error) {
	if c.calls.Add(1) == 1 {
		close(c.firstStarted)
		<-c.releaseFirst
		return 0, errors.New("first write failed")
	}
	return len(buffer), nil
}

func TestDialContextRetriesOnFailure(t *testing.T) {
	t.Parallel()
	var badCalls, goodCalls atomic.Int32
	bad := &mockOutbound{
		Adapter: outbound.NewAdapter("direct", "bad", []string{N.NetworkTCP}, nil),
		fail:    true,
		calls:   &badCalls,
	}
	good := &mockOutbound{
		Adapter: outbound.NewAdapter("direct", "good", []string{N.NetworkTCP}, nil),
		calls:   &goodCalls,
	}
	eng := engine.New([]string{"bad", "good"}, engine.Options{})
	// Pin sticky so the failing member is attempted first.
	eng.RememberHost("example.com", "bad")
	s := &Outbound{
		Adapter:        outbound.NewAdapter(C.TypeSmart, "smart", []string{N.NetworkTCP}, []string{"bad", "good"}),
		tags:           []string{"bad", "good"},
		outbounds:      map[string]adapter.Outbound{"bad": bad, "good": good},
		engine:         eng,
		interruptGroup: interrupt.NewGroup(),
		logger:         log.NewNOPFactory().Logger(),
	}

	conn, err := s.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	_ = conn.Close()

	if badCalls.Load() < 1 {
		t.Fatalf("expected bad outbound attempted, calls=%d", badCalls.Load())
	}
	if goodCalls.Load() < 1 {
		t.Fatalf("expected good outbound used, calls=%d", goodCalls.Load())
	}
	if s.Now() != "good" {
		t.Fatalf("selected=%q want good", s.Now())
	}
}

func TestDialContextSoftFailRetries(t *testing.T) {
	t.Parallel()
	var slowCalls, fastCalls atomic.Int32
	eng := engine.New([]string{"slow", "fast"}, engine.Options{SoftFailRatio: 1.5, SoftFailFloorMs: 80})
	eng.Record("slow", engine.OutcomeSuccess, 50)
	eng.Record("slow", engine.OutcomeSuccess, 50)
	eng.RememberHost("soft.example", "slow")

	slow := &mockOutbound{
		Adapter: outbound.NewAdapter("direct", "slow", []string{N.NetworkTCP}, nil),
		delay:   300 * time.Millisecond,
		calls:   &slowCalls,
	}
	fast := &mockOutbound{
		Adapter: outbound.NewAdapter("direct", "fast", []string{N.NetworkTCP}, nil),
		calls:   &fastCalls,
	}
	s := &Outbound{
		Adapter:        outbound.NewAdapter(C.TypeSmart, "smart", []string{N.NetworkTCP}, []string{"slow", "fast"}),
		tags:           []string{"slow", "fast"},
		outbounds:      map[string]adapter.Outbound{"slow": slow, "fast": fast},
		engine:         eng,
		interruptGroup: interrupt.NewGroup(),
		logger:         log.NewNOPFactory().Logger(),
	}

	conn, err := s.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("soft.example", 443))
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	_ = conn.Close()
	if slowCalls.Load() < 1 || fastCalls.Load() < 1 {
		t.Fatalf("expected soft-fail retry slow=%d fast=%d", slowCalls.Load(), fastCalls.Load())
	}
	if s.Now() != "fast" {
		t.Fatalf("selected=%q want fast after soft-fail", s.Now())
	}
}

func TestDialContextAllFail(t *testing.T) {
	t.Parallel()
	bad1 := &mockOutbound{
		Adapter: outbound.NewAdapter("direct", "a", []string{N.NetworkTCP}, nil),
		fail:    true,
	}
	bad2 := &mockOutbound{
		Adapter: outbound.NewAdapter("direct", "b", []string{N.NetworkTCP}, nil),
		fail:    true,
	}
	s := &Outbound{
		Adapter:        outbound.NewAdapter(C.TypeSmart, "smart", []string{N.NetworkTCP}, []string{"a", "b"}),
		tags:           []string{"a", "b"},
		outbounds:      map[string]adapter.Outbound{"a": bad1, "b": bad2},
		engine:         engine.New([]string{"a", "b"}, engine.Options{}),
		interruptGroup: interrupt.NewGroup(),
		logger:         log.NewNOPFactory().Logger(),
	}
	_, err := s.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
	if err == nil {
		t.Fatal("expected error when all members fail")
	}
}

func TestDialContextPublishesSanitizedFeedback(t *testing.T) {
	startSequence, _ := DialFeedbackDetailedSince(^uint64(0))
	tag := "feedback-failure"
	failing := &mockOutbound{
		Adapter: outbound.NewAdapter("direct", tag, []string{N.NetworkTCP}, nil),
		fail:    true,
	}
	s := &Outbound{
		Adapter:        outbound.NewAdapter(C.TypeSmart, "feedback-smart", []string{N.NetworkTCP}, []string{tag}),
		tags:           []string{tag},
		outbounds:      map[string]adapter.Outbound{tag: failing},
		engine:         engine.New([]string{tag}, engine.Options{}),
		interruptGroup: interrupt.NewGroup(),
		logger:         log.NewNOPFactory().Logger(),
	}

	_, err := s.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("private.example", 443))
	if err == nil {
		t.Fatal("expected dial failure")
	}

	_, events := DialFeedbackDetailedSince(startSequence)
	for _, event := range events {
		if event.Outbound != tag {
			continue
		}
		if event.Signal != dialFeedbackSignalTCP || event.Success || event.ErrorClass != "network" {
			t.Fatalf("unexpected event: %+v", event)
		}
		return
	}
	t.Fatalf("missing feedback for %q in %+v", tag, events)
}

func TestDialFeedbackPublishesOptInSignalsAndLegacyView(t *testing.T) {
	startSequence, _ := DialFeedbackDetailedSince(^uint64(0))
	tag := "feedback-signals"
	detour := &mockOutbound{
		Adapter:   outbound.NewAdapter("direct", tag, []string{N.NetworkTCP, N.NetworkUDP}, nil),
		handshake: true,
	}
	s := &Outbound{
		Adapter:        outbound.NewAdapter(C.TypeSmart, "feedback-smart", []string{N.NetworkTCP, N.NetworkUDP}, []string{tag}),
		tags:           []string{tag},
		outbounds:      map[string]adapter.Outbound{tag: detour},
		engine:         engine.New([]string{tag}, engine.Options{}),
		interruptGroup: interrupt.NewGroup(),
		logger:         log.NewNOPFactory().Logger(),
	}

	conn, err := s.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("private.example", 443))
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	sequenceAfterDial, dialEvents := DialFeedbackDetailedSince(startSequence)
	if len(dialEvents) != 1 || dialEvents[0].Signal != dialFeedbackSignalTCP || !dialEvents[0].Success {
		t.Fatalf("unexpected TCP signal events: %+v", dialEvents)
	}
	legacySequence, legacyBeforeHandshake := DialFeedbackSince(startSequence)
	if legacySequence != sequenceAfterDial || legacyBeforeHandshake == nil || len(legacyBeforeHandshake) != 0 {
		t.Fatalf("legacy before handshake sequence=%d events=%+v", legacySequence, legacyBeforeHandshake)
	}

	if _, err = conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	response := make([]byte, 2)
	if _, err = conn.Read(response); err != nil {
		t.Fatalf("read: %v", err)
	}
	_ = conn.Close()

	sequenceAfterRead, postDialEvents := DialFeedbackDetailedSince(sequenceAfterDial)
	if len(postDialEvents) != 2 ||
		postDialEvents[0].Signal != dialFeedbackSignalHandshake ||
		postDialEvents[1].Signal != dialFeedbackSignalFirstByte ||
		!postDialEvents[0].Success ||
		!postDialEvents[1].Success {
		t.Fatalf("unexpected handshake/first-byte events: %+v", postDialEvents)
	}
	legacySequence, legacyAfterHandshake := DialFeedbackSince(startSequence)
	if legacySequence != sequenceAfterRead ||
		len(legacyAfterHandshake) != 1 ||
		legacyAfterHandshake[0].Signal != dialFeedbackSignalHandshake {
		t.Fatalf("unexpected legacy events: %+v", legacyAfterHandshake)
	}

	_, err = s.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("private.example", 443))
	if err == nil {
		t.Fatal("expected UDP failure")
	}
	_, udpEvents := DialFeedbackDetailedSince(sequenceAfterRead)
	if len(udpEvents) != 1 ||
		udpEvents[0].Signal != dialFeedbackSignalUDP ||
		udpEvents[0].Success ||
		udpEvents[0].ErrorClass != "network" {
		t.Fatalf("unexpected UDP events: %+v", udpEvents)
	}
}

func TestFirstByteObserveConn(t *testing.T) {
	t.Parallel()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	observed := make(chan error, 1)
	conn := &firstByteObserveConn{
		ExtendedConn: bufio.NewExtendedConn(client),
		onFirstByte: func(err error, _ time.Duration) {
			observed <- err
		},
	}
	go func() {
		buffer := make([]byte, 4)
		_, _ = server.Read(buffer)
		_, _ = server.Write([]byte("ok"))
	}()
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buffer := make([]byte, 2)
	if _, err := conn.Read(buffer); err != nil {
		t.Fatalf("read: %v", err)
	}
	select {
	case err := <-observed:
		if err != nil {
			t.Fatalf("first byte marked failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first-byte observation timed out")
	}
}

func TestFirstWriteObserveConnKeepsFirstConcurrentWrite(t *testing.T) {
	t.Parallel()
	base := &reorderedWriteConn{
		firstStarted: make(chan struct{}),
		releaseFirst: make(chan struct{}),
	}
	observed := make(chan error, 1)
	conn := &firstWriteObserveConn{
		Conn: base,
		onFirstWrite: func(err error, _ time.Duration) {
			observed <- err
		},
	}
	firstResult := make(chan error, 1)
	go func() {
		_, err := conn.Write([]byte("first"))
		firstResult <- err
	}()
	<-base.firstStarted
	if _, err := conn.Write([]byte("second")); err != nil {
		t.Fatalf("second write: %v", err)
	}
	close(base.releaseFirst)
	if err := <-firstResult; err == nil {
		t.Fatal("first write unexpectedly succeeded")
	}
	select {
	case err := <-observed:
		if err == nil || err.Error() != "first write failed" {
			t.Fatalf("observed error=%v, want first write failure", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first-write observation timed out")
	}
}
