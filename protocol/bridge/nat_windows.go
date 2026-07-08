//go:build windows && (amd64 || 386)

package bridge

import (
	"net/netip"
	"sync"
	"time"
)

// icmpTable records liveness of ICMP echo flows by (identifier, remote
// address). The identifier passes through untranslated — the dispatcher NAT
// already set it to the selector — and Windows ping.exe uses a constant
// identifier, so the remote address is needed to tell a bridged reply from the
// host's own ping.
type icmpTable struct {
	access    sync.Mutex
	timeout   time.Duration
	active    map[icmpFlowKey]time.Time
	lastSweep time.Time
}

type icmpFlowKey struct {
	identifier uint16
	remote     netip.Addr
}

func newICMPTable(timeout time.Duration) *icmpTable {
	return &icmpTable{
		timeout: timeout,
		active:  make(map[icmpFlowKey]time.Time),
	}
}

func (t *icmpTable) register(identifier uint16, remote netip.Addr) {
	now := time.Now()
	key := icmpFlowKey{identifier: identifier, remote: remote}
	t.access.Lock()
	defer t.access.Unlock()
	if now.Sub(t.lastSweep) >= t.timeout {
		t.lastSweep = now
		for flow, lastActive := range t.active {
			if now.Sub(lastActive) >= t.timeout {
				delete(t.active, flow)
			}
		}
	}
	t.active[key] = now
}

func (t *icmpTable) isActive(identifier uint16, remote netip.Addr) bool {
	now := time.Now()
	key := icmpFlowKey{identifier: identifier, remote: remote}
	t.access.Lock()
	defer t.access.Unlock()
	lastActive, loaded := t.active[key]
	if !loaded {
		return false
	}
	if now.Sub(lastActive) >= t.timeout {
		delete(t.active, key)
		return false
	}
	t.active[key] = now
	return true
}
