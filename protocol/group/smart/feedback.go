package smart

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const dialFeedbackCapacity = 256

var dialFeedbackInstance = newDialFeedbackInstance()

// DialFeedbackEvent is a privacy-preserving Smart member dial outcome.
// Destination details and raw errors are deliberately excluded.
type DialFeedbackEvent struct {
	Sequence   uint64 `json:"sequence"`
	Group      string `json:"group,omitempty"`
	Outbound   string `json:"outbound"`
	Network    string `json:"network,omitempty"`
	Success    bool   `json:"success"`
	DurationMs int64  `json:"durationMs"`
	Timestamp  int64  `json:"timestamp"`
	ErrorClass string `json:"errorClass"`
}

type dialFeedbackBuffer struct {
	mu       sync.RWMutex
	events   [dialFeedbackCapacity]DialFeedbackEvent
	sequence uint64
	count    int
}

var globalDialFeedback dialFeedbackBuffer

func newDialFeedbackInstance() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("smart: secure random source unavailable")
	}
	return hex.EncodeToString(value[:])
}

func (b *dialFeedbackBuffer) record(group string, outbound string, network string, success bool, duration time.Duration, errorClass string) {
	var durationMs int64
	if duration > 0 {
		durationMs = int64((duration + 500*time.Microsecond) / time.Millisecond)
		if durationMs < 1 {
			durationMs = 1
		}
	}
	if success {
		errorClass = ""
	} else {
		switch errorClass {
		case "canceled", "network", "soft-fail", "timeout":
		default:
			errorClass = "unknown"
		}
	}
	timestamp := time.Now().UnixMilli()

	b.mu.Lock()
	b.sequence++
	event := DialFeedbackEvent{
		Sequence:   b.sequence,
		Group:      group,
		Outbound:   outbound,
		Network:    network,
		Success:    success,
		DurationMs: durationMs,
		Timestamp:  timestamp,
		ErrorClass: errorClass,
	}
	b.events[(event.Sequence-1)%dialFeedbackCapacity] = event
	if b.count < dialFeedbackCapacity {
		b.count++
	}
	b.mu.Unlock()
}

func (b *dialFeedbackBuffer) snapshot(since uint64) (uint64, []DialFeedbackEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	sequence := b.sequence
	if b.count == 0 || since >= sequence {
		return sequence, []DialFeedbackEvent{}
	}

	first := sequence - uint64(b.count) + 1
	start := first
	if since >= first {
		start = since + 1
	}
	events := make([]DialFeedbackEvent, 0, int(sequence-start+1))
	for current := start; current <= sequence; current++ {
		events = append(events, b.events[(current-1)%dialFeedbackCapacity])
	}
	return sequence, events
}

func recordDialFeedback(group string, outbound string, network string, success bool, duration time.Duration, errorClass string) {
	globalDialFeedback.record(group, outbound, network, success, duration, errorClass)
}

// DialFeedbackSince returns bounded incremental Smart dial outcomes.
func DialFeedbackSince(since uint64) (uint64, []DialFeedbackEvent) {
	return globalDialFeedback.snapshot(since)
}

// DialFeedbackInstance returns the privacy-safe identifier for this process.
func DialFeedbackInstance() string {
	return dialFeedbackInstance
}
