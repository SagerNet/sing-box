package engine

import (
	"math"
	"strings"
	"sync"
	"time"
)

// Engine is the shared Smart decision state for one group instance.
type Engine struct {
	mu         sync.Mutex
	opts       Options
	members    map[string]*MemberStats
	udpMembers map[string]*MemberStats
	order      []string
	hosts      map[string]hostSticky
	targets    map[string]*targetStats
	preferred  string // API / app preferred member (soft pin)
}

type hostSticky struct {
	Tag       string
	UpdatedAt time.Time
}

type targetStats struct {
	UpdatedAt time.Time
	Members   map[string]*MemberStats
}

// New creates an engine for the given member tags (outbound names).
func New(tags []string, opts Options) *Engine {
	opts = opts.withDefaults()
	e := &Engine{
		opts:       opts,
		members:    make(map[string]*MemberStats, len(tags)),
		udpMembers: make(map[string]*MemberStats, len(tags)),
		order:      nil,
		hosts:      make(map[string]hostSticky),
		targets:    make(map[string]*targetStats),
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
	nextUDP := make(map[string]*MemberStats, len(tags))
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
		if m, ok := e.udpMembers[tag]; ok {
			nextUDP[tag] = m
		} else {
			nextUDP[tag] = &MemberStats{Tag: tag, Weight: 1, Alive: true}
		}
		order = append(order, tag)
	}
	e.members = next
	e.udpMembers = nextUDP
	e.order = order
	for key, target := range e.targets {
		for tag := range target.Members {
			if _, exists := next[tag]; !exists {
				delete(target.Members, tag)
			}
		}
		if len(target.Members) == 0 {
			delete(e.targets, key)
		}
	}
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
	if udp := e.udpMembers[tag]; udp != nil {
		udp.Weight = weight
	}
	for _, target := range e.targets {
		if member := target.Members[tag]; member != nil {
			member.Weight = weight
		}
	}
}

func (e *Engine) memberMapLocked(network Network) map[string]*MemberStats {
	if network == NetworkUDP {
		return e.udpMembers
	}
	return e.members
}

func targetKey(target string, network Network) string {
	target = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(target), "."))
	if target == "" {
		return ""
	}
	return string(network) + "\x00" + target
}

func (e *Engine) ensureTargetLocked(target string, network Network, now time.Time) *targetStats {
	key := targetKey(target, network)
	if key == "" {
		return nil
	}
	if state := e.targets[key]; state != nil {
		state.UpdatedAt = now
		return state
	}
	if len(e.targets) >= e.opts.MaxTargets {
		var oldestKey string
		var oldest time.Time
		for candidate, state := range e.targets {
			if oldestKey == "" || state.UpdatedAt.Before(oldest) {
				oldestKey, oldest = candidate, state.UpdatedAt
			}
		}
		delete(e.targets, oldestKey)
		delete(e.hosts, oldestKey)
	}
	state := &targetStats{UpdatedAt: now, Members: make(map[string]*MemberStats)}
	e.targets[key] = state
	return state
}

// SetURLTestPrior seeds latency from a URL health check (may be refreshed).
func (e *Engine) SetURLTestPrior(tag string, latencyMs uint16) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, members := range []map[string]*MemberStats{e.members, e.udpMembers} {
		m := members[tag]
		if m == nil {
			continue
		}
		m.URLTestLatencyMs = latencyMs
		m.HasURLTestPrior = true
		if m.Samples == 0 {
			m.EwmaMs = float64(latencyMs)
		}
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
	e.RecordFor("", NetworkTCP, tag, outcome, rttMs)
}

// RecordFor updates protocol-wide health and a bounded target-specific view.
func (e *Engine) RecordFor(target string, network Network, tag string, outcome Outcome, rttMs float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	m := e.memberMapLocked(network)[tag]
	if m == nil {
		return
	}
	now := e.opts.Now()
	targetState := e.ensureTargetLocked(target, network, now)
	// A destination-specific failure may be blocking or routing policy, not a
	// globally bad proxy. Successful dials still improve the global prior.
	if targetState == nil || outcome == OutcomeSuccess {
		e.recordLocked(m, outcome, rttMs, now)
	}
	if targetState != nil {
		targetMember := targetState.Members[tag]
		if targetMember == nil {
			targetMember = &MemberStats{Tag: tag, Weight: m.Weight, Alive: true}
			targetState.Members[tag] = targetMember
		}
		e.recordLocked(targetMember, outcome, rttMs, now)
	}
}

func (e *Engine) recordLocked(m *MemberStats, outcome Outcome, rttMs float64, now time.Time) {
	e.decayPenaltyLocked(m, now)
	m.Attempts++
	if m.Attempts > 1000000 {
		m.Attempts = 1000000
	}

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

// RecordFirstByteFor records request-to-first-response latency. Destination
// observations stay contextual because a slow or blocked site is not a bad proxy.
func (e *Engine) RecordFirstByteFor(target string, network Network, tag string, success bool, latencyMs float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	m := e.memberMapLocked(network)[tag]
	if m == nil {
		return
	}
	now := e.opts.Now()
	targetState := e.ensureTargetLocked(target, network, now)
	if targetState == nil {
		e.recordFirstByteLocked(m, success, latencyMs)
	}
	if targetState != nil {
		targetMember := targetState.Members[tag]
		if targetMember == nil {
			targetMember = &MemberStats{Tag: tag, Weight: m.Weight, Alive: true}
			targetState.Members[tag] = targetMember
		}
		e.recordFirstByteLocked(targetMember, success, latencyMs)
	}
}

func (e *Engine) recordFirstByteLocked(m *MemberStats, success bool, latencyMs float64) {
	m.FirstByteAttempts++
	if m.FirstByteAttempts > 1000000 {
		m.FirstByteAttempts = 1000000
	}
	if !success {
		m.FirstByteFailRate += (1 - m.FirstByteFailRate) * 0.25
		return
	}
	if latencyMs < 0 {
		latencyMs = 0
	}
	if m.FirstByteSamples == 0 {
		m.FirstByteEwmaMs = latencyMs
	} else {
		m.FirstByteEwmaMs = m.FirstByteEwmaMs*0.7 + latencyMs*0.3
	}
	m.FirstByteSamples++
	if m.FirstByteSamples > 1000 {
		m.FirstByteSamples = 1000
	}
	m.FirstByteFailRate *= 0.75
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
	return e.SnapshotFor(NetworkTCP)
}

func (e *Engine) SnapshotFor(network Network) []MemberStats {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]MemberStats, 0, len(e.order))
	now := e.opts.Now()
	for _, tag := range e.order {
		m := e.memberMapLocked(network)[tag]
		if m == nil {
			continue
		}
		e.decayPenaltyLocked(m, now)
		cp := *m
		out = append(out, cp)
	}
	return out
}
