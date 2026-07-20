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
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type mockOutbound struct {
	outbound.Adapter
	fail    bool
	delay   time.Duration
	calls   *atomic.Int32
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
	go c2.Close()
	return c1, nil
}

func (m *mockOutbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("udp not implemented")
}

func TestDialContextRetriesOnFailure(t *testing.T) {
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
	eng := engine.New([]string{"bad", "good"}, engine.Options{TopK: 1})
	// Pin sticky so the failing member is attempted first (order independent of softmax).
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
