package smart

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

const (
	dialFeedbackCapacity        = 256
	dialFeedbackSignalTCP       = "tcp"
	dialFeedbackSignalUDP       = "udp"
	dialFeedbackSignalHandshake = "handshake"
	dialFeedbackSignalFirstByte = "first-byte"
)

var dialFeedbackInstance = newDialFeedbackInstance()

// DialFeedbackEvent is a privacy-preserving Smart member dial outcome.
// Destination details and raw errors are deliberately excluded.
type DialFeedbackEvent struct {
	Sequence         uint64 `json:"sequence"`
	Group            string `json:"group,omitempty"`
	Outbound         string `json:"outbound"`
	Network          string `json:"network,omitempty"`
	Signal           string `json:"signal"`
	Success          bool   `json:"success"`
	DurationMs       int64  `json:"durationMs"`
	Timestamp        int64  `json:"timestamp"`
	ErrorClass       string `json:"errorClass"`
	legacy           bool   `json:"-"`
	legacyDurationMs int64  `json:"-"`
}

type dialFeedbackBuffer struct {
	mu           sync.RWMutex
	events       [dialFeedbackCapacity]DialFeedbackEvent
	sequence     uint64
	count        int
	legacyEvents [dialFeedbackCapacity]DialFeedbackEvent
	legacyStart  int
	legacyCount  int
}

var globalDialFeedback dialFeedbackBuffer

func newDialFeedbackInstance() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("smart: secure random source unavailable")
	}
	return hex.EncodeToString(value[:])
}

func normalizeDialFeedbackSignal(signal string, network string) string {
	switch signal {
	case dialFeedbackSignalTCP, dialFeedbackSignalUDP, dialFeedbackSignalHandshake, dialFeedbackSignalFirstByte:
		return signal
	}
	if strings.EqualFold(network, "udp") {
		return dialFeedbackSignalUDP
	}
	return dialFeedbackSignalTCP
}

func normalizeDialFeedbackDuration(duration time.Duration) int64 {
	if duration <= 0 {
		return 0
	}
	durationMs := int64((duration + 500*time.Microsecond) / time.Millisecond)
	if durationMs < 1 {
		return 1
	}
	return durationMs
}

func (b *dialFeedbackBuffer) record(
	group string,
	outbound string,
	network string,
	signal string,
	legacy bool,
	success bool,
	duration time.Duration,
	errorClass string,
) {
	b.recordProjected(
		group,
		outbound,
		network,
		signal,
		legacy,
		success,
		duration,
		duration,
		errorClass,
	)
}

func (b *dialFeedbackBuffer) recordProjected(
	group string,
	outbound string,
	network string,
	signal string,
	legacy bool,
	success bool,
	duration time.Duration,
	legacyDuration time.Duration,
	errorClass string,
) {
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
		Sequence:         b.sequence,
		Group:            group,
		Outbound:         outbound,
		Network:          network,
		Signal:           normalizeDialFeedbackSignal(signal, network),
		Success:          success,
		DurationMs:       normalizeDialFeedbackDuration(duration),
		Timestamp:        timestamp,
		ErrorClass:       errorClass,
		legacy:           legacy,
		legacyDurationMs: normalizeDialFeedbackDuration(legacyDuration),
	}
	b.events[(event.Sequence-1)%dialFeedbackCapacity] = event
	if b.count < dialFeedbackCapacity {
		b.count++
	}
	if legacy {
		legacyEvent := event
		legacyEvent.DurationMs = event.legacyDurationMs
		index := (b.legacyStart + b.legacyCount) % dialFeedbackCapacity
		if b.legacyCount == dialFeedbackCapacity {
			index = b.legacyStart
			b.legacyStart = (b.legacyStart + 1) % dialFeedbackCapacity
		} else {
			b.legacyCount++
		}
		b.legacyEvents[index] = legacyEvent
	}
	b.mu.Unlock()
}

func (b *dialFeedbackBuffer) snapshotDetailed(since uint64) (uint64, []DialFeedbackEvent) {
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

func (b *dialFeedbackBuffer) snapshotLegacy(since uint64) (uint64, []DialFeedbackEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	sequence := b.sequence
	if b.legacyCount == 0 || since >= sequence {
		return sequence, []DialFeedbackEvent{}
	}
	capacity := 0
	for index := 0; index < b.legacyCount; index++ {
		event := b.legacyEvents[(b.legacyStart+index)%dialFeedbackCapacity]
		if event.Sequence > since {
			capacity++
		}
	}
	events := make([]DialFeedbackEvent, 0, capacity)
	for index := 0; index < b.legacyCount; index++ {
		event := b.legacyEvents[(b.legacyStart+index)%dialFeedbackCapacity]
		if event.Sequence > since {
			events = append(events, event)
		}
	}
	return sequence, events
}

func (b *dialFeedbackBuffer) snapshot(since uint64, includeSignals bool) (uint64, []DialFeedbackEvent) {
	if includeSignals {
		return b.snapshotDetailed(since)
	}
	return b.snapshotLegacy(since)
}

func recordDialFeedback(
	group string,
	outbound string,
	network string,
	signal string,
	legacy bool,
	success bool,
	duration time.Duration,
	errorClass string,
) {
	globalDialFeedback.record(group, outbound, network, signal, legacy, success, duration, errorClass)
}

func recordProjectedDialFeedback(
	group string,
	outbound string,
	network string,
	signal string,
	legacy bool,
	success bool,
	duration time.Duration,
	legacyDuration time.Duration,
	errorClass string,
) {
	globalDialFeedback.recordProjected(
		group,
		outbound,
		network,
		signal,
		legacy,
		success,
		duration,
		legacyDuration,
		errorClass,
	)
}

// DialFeedbackSince returns the bounded legacy-compatible outcome stream.
func DialFeedbackSince(since uint64) (uint64, []DialFeedbackEvent) {
	return globalDialFeedback.snapshotLegacy(since)
}

// DialFeedbackDetailedSince returns the bounded stage-specific outcome stream.
func DialFeedbackDetailedSince(since uint64) (uint64, []DialFeedbackEvent) {
	return globalDialFeedback.snapshotDetailed(since)
}

// DialFeedbackInstance returns the privacy-safe identifier for this process.
func DialFeedbackInstance() string {
	return dialFeedbackInstance
}
