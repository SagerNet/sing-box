package engine

import (
	"math"
	"sync"
	"time"
)

// Engine is the shared Smart decision state for one group instance.
type Engine struct {
	mu        sync.Mutex
	opts      Options
	members   map[string]*MemberStats
	order     []string
	hosts     map[string]hostSticky
	preferred string // API / app preferred member (soft pin)
}

type hostSticky struct {
	Tag       string
	UpdatedAt time.Time
}

// New creates an engine for the given member tags (outbound names).
func New(tags []string, opts Options) *Engine {
	opts = opts.withDefaults()
	e := &Engine{
		opts:    opts,
		members: make(map[string]*MemberStats, len(tags)),
		order:   nil,
		hosts:   make(map[string]hostSticky),
	}
	e.syncMembersLocked(tags)
	return e
}

// SyncMembers updates the member set (e.g. provider refresh). Keeps stats for
// tags that still exist; drops removed tags; adds new ones cold.
func (e *Engine) SyncMembers(tags []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.syncMembersLocked(tags)
}

func (e *Engine) syncMembersLocked(tags []string) {
	next := make(map[string]*MemberStats, len(tags))
	order := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		if m, ok := e.members[tag]; ok {
			next[tag] = m
		} else {
			next[tag] = &MemberStats{Tag: tag, Weight: 1, Alive: true}
		}
		order = append(order, tag)
	}
	e.members = next
	e.order = order
	if e.preferred != "" {
		if _, ok := e.members[e.preferred]; !ok {
			e.preferred = ""
		}
	}
}

// SetWeight applies a policy-priority style coefficient (Surge-compatible idea).
func (e *Engine) SetWeight(tag string, weight float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	m := e.members[tag]
	if m == nil {
		return
	}
	if weight <= 0 {
		weight = 0.01
	}
	m.Weight = weight
}

// SetURLTestPrior seeds latency from a URL health check (may be refreshed).
func (e *Engine) SetURLTestPrior(tag string, latencyMs uint16) {
	e.mu.Lock()
	defer e.mu.Unlock()
	m := e.members[tag]
	if m == nil {
		return
	}
	m.URLTestLatencyMs = latencyMs
	m.HasURLTestPrior = true
	if m.Samples == 0 {
		m.EwmaMs = float64(latencyMs)
	}
}

// SetPreferred pins a member for subsequent Select (Clash API / app smart pick).
// Empty tag clears the pin.
func (e *Engine) SetPreferred(tag string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if tag != "" {
		if _, ok := e.members[tag]; !ok {
			return
		}
	}
	e.preferred = tag
}

// Preferred returns the current API/app preferred member tag.
func (e *Engine) Preferred() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.preferred
}

// SoftFailThresholdMs returns the soft-fail ceiling for a member (Surge ×1.5 idea).
func (e *Engine) SoftFailThresholdMs(tag string) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	m := e.members[tag]
	if m == nil || m.EwmaMs <= 0 {
		return 5000
	}
	return math.Max(m.EwmaMs*e.opts.SoftFailRatio, m.EwmaMs+e.opts.SoftFailFloorMs)
}

// Record dial/handshake outcome for a member.
func (e *Engine) Record(tag string, outcome Outcome, rttMs float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	m := e.members[tag]
	if m == nil {
		return
	}
	now := e.opts.Now()
	e.decayPenaltyLocked(m, now)

	switch outcome {
	case OutcomeSuccess:
		if rttMs < 0 {
			rttMs = 0
		}
		if m.Samples == 0 {
			m.EwmaMs = rttMs
		} else {
			diff := math.Abs(rttMs - m.EwmaMs)
			m.JitterMs = m.JitterMs*0.7 + diff*0.3
			m.EwmaMs = m.EwmaMs*0.7 + rttMs*0.3
		}
		m.Samples++
		if m.Samples > 1000 {
			m.Samples = 1000
		}
		m.FailureRate *= 0.75
		m.ConsecutiveFails = 0
		m.LastSuccess = now
		m.SuccessSinceFail++
		// Strong wipe on success (Surge: success largely erases penalty).
		m.Penalty *= 0.25
		if m.Penalty < 0.05 {
			m.Penalty = 0
		}
		m.Alive = true
	case OutcomeFailure, OutcomeSoftFail:
		m.FailureRate = m.FailureRate*0.75 + 0.25
		m.ConsecutiveFails++
		if m.ConsecutiveFails > 16 {
			m.ConsecutiveFails = 16
		}
		m.LastFailure = now
		m.SuccessSinceFail = 0
		if m.Penalty <= 0 {
			m.Penalty = e.opts.PenaltyBase
		} else {
			m.Penalty = math.Min(e.opts.PenaltyMax, m.Penalty*e.opts.PenaltyGrowth)
		}
		if outcome == OutcomeFailure {
			m.Alive = m.ConsecutiveFails < 3
		}
		if rttMs > 0 && m.Samples > 0 {
			// Soft-fail still updates EWMA gently so thresholds track reality.
			m.EwmaMs = m.EwmaMs*0.9 + rttMs*0.1
		}
	}
}

func (e *Engine) decayPenaltyLocked(m *MemberStats, now time.Time) {
	if m.Penalty <= 0 || m.LastFailure.IsZero() {
		return
	}
	elapsed := now.Sub(m.LastFailure)
	if elapsed <= 0 {
		return
	}
	// Exponential half-life decay.
	halfLives := float64(elapsed) / float64(e.opts.PenaltyHalfLife)
	m.Penalty *= math.Pow(0.5, halfLives)
	if m.Penalty < 0.05 {
		m.Penalty = 0
	}
}

// Snapshot returns a copy of member stats (for tests / diagnostics).
func (e *Engine) Snapshot() []MemberStats {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]MemberStats, 0, len(e.order))
	now := e.opts.Now()
	for _, tag := range e.order {
		m := e.members[tag]
		if m == nil {
			continue
		}
		e.decayPenaltyLocked(m, now)
		cp := *m
		out = append(out, cp)
	}
	return out
}
