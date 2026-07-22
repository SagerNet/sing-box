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
	DefaultMaxHosts        = 4096
	DefaultMaxTargets      = 1024
	DefaultMaxMembers      = 512
	DefaultExplorationMs   = 90.0
)

type Network string

const (
	NetworkTCP Network = "tcp"
	NetworkUDP Network = "udp"
)

// MemberStats is per-outbound health used for selection.
type MemberStats struct {
	Tag               string
	Samples           int
	EwmaMs            float64
	JitterMs          float64
	FailureRate       float64
	ConsecutiveFails  int
	Penalty           float64
	LastSuccess       time.Time
	LastFailure       time.Time
	SuccessSinceFail  int
	Weight            float64 // policy-priority style; 1 = neutral, <1 preferred
	Alive             bool
	URLTestLatencyMs  uint16 // optional prior from URL test
	HasURLTestPrior   bool
	Attempts          int
	FirstByteAttempts int
	FirstByteSamples  int
	FirstByteEwmaMs   float64
	FirstByteFailRate float64
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
	MaxHosts        int
	MaxTargets      int
	ExplorationMs   float64
	RTTWeight       float64
	FirstByteWeight float64
	JitterWeight    float64
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
	if o.MaxHosts <= 0 {
		o.MaxHosts = DefaultMaxHosts
	}
	if o.MaxTargets <= 0 {
		o.MaxTargets = DefaultMaxTargets
	}
	if o.ExplorationMs <= 0 {
		o.ExplorationMs = DefaultExplorationMs
	}
	if o.RTTWeight <= 0 {
		o.RTTWeight = 0.45
	}
	if o.FirstByteWeight <= 0 {
		o.FirstByteWeight = 0.35
	}
	if o.JitterWeight <= 0 {
		o.JitterWeight = 0.2
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return o
}

// OptionsForMode keeps user-facing modes small while preserving one engine.
func OptionsForMode(mode string) Options {
	switch mode {
	case "latency":
		return Options{
			SoftFailRatio: 1.35, SoftFailFloorMs: 60,
			PenaltyHalfLife: 3 * time.Minute, HostStickyTTL: 4 * time.Minute,
			ExplorationMs: 120, RTTWeight: 0.55, FirstByteWeight: 0.35, JitterWeight: 0.1,
		}
	case "stable":
		return Options{
			SoftFailRatio: 1.8, SoftFailFloorMs: 120,
			PenaltyGrowth: 2.4, PenaltyHalfLife: 10 * time.Minute,
			HostStickyTTL: 30 * time.Minute,
			ExplorationMs: 45, RTTWeight: 0.35, FirstByteWeight: 0.3, JitterWeight: 0.35,
		}
	default:
		return Options{}
	}
}

// Outcome of a dial/handshake attempt.
type Outcome int

const (
	OutcomeSuccess Outcome = iota
	OutcomeFailure
	OutcomeSoftFail
)
