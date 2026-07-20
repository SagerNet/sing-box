// Package engine implements a kernel-agnostic Smart group decision engine
// inspired by Surge Smart policy groups. Adapters in sing-box / mihomo wrap
// Dial paths and call this package; do not import kernel packages here.
package engine

import "time"

// Default knobs (tunable later via options).
const (
	DefaultSoftFailRatio   = 1.5
	DefaultSoftFailFloorMs = 80
	DefaultPenaltyBase     = 1.0
	DefaultPenaltyGrowth   = 2.0
	DefaultPenaltyMax      = 64.0
	DefaultPenaltyHalfLife = 5 * time.Minute
	DefaultHostStickyTTL   = 10 * time.Minute
	DefaultTopK            = 4
	DefaultMaxHosts        = 4096
	DefaultMaxMembers      = 512
)

// MemberStats is per-outbound health used for selection.
type MemberStats struct {
	Tag              string
	Samples          int
	EwmaMs           float64
	JitterMs         float64
	FailureRate      float64
	ConsecutiveFails int
	Penalty          float64
	LastSuccess      time.Time
	LastFailure      time.Time
	SuccessSinceFail int
	Weight           float64 // policy-priority style; 1 = neutral, <1 preferred
	Alive            bool
	URLTestLatencyMs uint16 // optional prior from URL test
	HasURLTestPrior  bool
}

// Options configures an Engine.
type Options struct {
	SoftFailRatio   float64
	SoftFailFloorMs float64
	PenaltyBase     float64
	PenaltyGrowth   float64
	PenaltyMax      float64
	PenaltyHalfLife time.Duration
	HostStickyTTL   time.Duration
	TopK            int
	MaxHosts        int
	ExploreProb     float64 // 0..1 chance to probe non-sticky candidate
	Now             func() time.Time
}

func (o Options) withDefaults() Options {
	if o.SoftFailRatio <= 0 {
		o.SoftFailRatio = DefaultSoftFailRatio
	}
	if o.SoftFailFloorMs <= 0 {
		o.SoftFailFloorMs = DefaultSoftFailFloorMs
	}
	if o.PenaltyBase <= 0 {
		o.PenaltyBase = DefaultPenaltyBase
	}
	if o.PenaltyGrowth <= 1 {
		o.PenaltyGrowth = DefaultPenaltyGrowth
	}
	if o.PenaltyMax <= 0 {
		o.PenaltyMax = DefaultPenaltyMax
	}
	if o.PenaltyHalfLife <= 0 {
		o.PenaltyHalfLife = DefaultPenaltyHalfLife
	}
	if o.HostStickyTTL <= 0 {
		o.HostStickyTTL = DefaultHostStickyTTL
	}
	if o.TopK <= 0 {
		o.TopK = DefaultTopK
	}
	if o.MaxHosts <= 0 {
		o.MaxHosts = DefaultMaxHosts
	}
	if o.ExploreProb < 0 {
		o.ExploreProb = 0.15
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return o
}

// Outcome of a dial/handshake attempt.
type Outcome int

const (
	OutcomeSuccess Outcome = iota
	OutcomeFailure
	OutcomeSoftFail
)
