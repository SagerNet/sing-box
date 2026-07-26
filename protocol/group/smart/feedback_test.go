package smart

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDialFeedbackBufferIsBoundedAndIncremental(t *testing.T) {
	t.Parallel()

	var buffer dialFeedbackBuffer
	for index := 0; index < dialFeedbackCapacity+4; index++ {
		buffer.record(
			"smart",
			"node",
			"tcp",
			dialFeedbackSignalTCP,
			true,
			index%2 == 0,
			time.Duration(index)*time.Millisecond,
			"unknown",
		)
	}

	sequence, events := buffer.snapshot(0, true)
	if sequence != dialFeedbackCapacity+4 {
		t.Fatalf("sequence=%d", sequence)
	}
	if len(events) != dialFeedbackCapacity {
		t.Fatalf("events=%d", len(events))
	}
	if events[0].Sequence != 5 || events[len(events)-1].Sequence != sequence {
		t.Fatalf("unexpected retained range %d..%d", events[0].Sequence, events[len(events)-1].Sequence)
	}

	_, incremental := buffer.snapshot(sequence-2, true)
	if len(incremental) != 2 || incremental[0].Sequence != sequence-1 || incremental[1].Sequence != sequence {
		t.Fatalf("unexpected incremental events: %+v", incremental)
	}

	current, future := buffer.snapshot(sequence+10, true)
	if current != sequence || len(future) != 0 {
		t.Fatalf("future cursor response sequence=%d events=%d", current, len(future))
	}
}

func TestDialFeedbackBufferFiltersLegacyAndKeepsGlobalCursor(t *testing.T) {
	t.Parallel()

	var buffer dialFeedbackBuffer
	buffer.record("smart", "node", "tcp", dialFeedbackSignalTCP, false, true, time.Millisecond, "")
	buffer.record("smart", "node", "tcp", dialFeedbackSignalHandshake, true, true, 2*time.Millisecond, "")
	buffer.record("smart", "node", "tcp", dialFeedbackSignalFirstByte, false, true, 3*time.Millisecond, "")
	buffer.record("smart", "node", "udp", dialFeedbackSignalUDP, true, false, 4*time.Millisecond, "network")

	sequence, legacy := buffer.snapshot(0, false)
	if sequence != 4 {
		t.Fatalf("legacy sequence=%d", sequence)
	}
	if len(legacy) != 2 || legacy[0].Sequence != 2 || legacy[1].Sequence != 4 {
		t.Fatalf("unexpected legacy events: %+v", legacy)
	}
	if legacy[0].Signal != dialFeedbackSignalHandshake || legacy[1].Signal != dialFeedbackSignalUDP {
		t.Fatalf("unexpected legacy signals: %+v", legacy)
	}

	_, detailed := buffer.snapshot(0, true)
	if len(detailed) != 4 {
		t.Fatalf("detailed events=%d", len(detailed))
	}
	wantSignals := []string{
		dialFeedbackSignalTCP,
		dialFeedbackSignalHandshake,
		dialFeedbackSignalFirstByte,
		dialFeedbackSignalUDP,
	}
	for index, want := range wantSignals {
		if detailed[index].Signal != want {
			t.Fatalf("detailed[%d].signal=%q want %q", index, detailed[index].Signal, want)
		}
	}

	buffer.record("smart", "node", "tcp", "unsafe arbitrary phase", false, true, time.Millisecond, "")
	sequence, hidden := buffer.snapshot(4, false)
	if sequence != 5 || hidden == nil || len(hidden) != 0 {
		t.Fatalf("hidden legacy response sequence=%d events=%v", sequence, hidden)
	}
	_, normalized := buffer.snapshot(4, true)
	if len(normalized) != 1 || normalized[0].Signal != dialFeedbackSignalTCP {
		t.Fatalf("normalized signal events=%+v", normalized)
	}
}

func TestDialFeedbackBufferRetainsIndependentLegacyWindowAndDurationProjection(t *testing.T) {
	t.Parallel()

	var buffer dialFeedbackBuffer
	for attempt := 0; attempt < 300; attempt++ {
		buffer.record(
			"smart",
			"node",
			"tcp",
			dialFeedbackSignalTCP,
			false,
			true,
			time.Millisecond,
			"",
		)
		buffer.recordProjected(
			"smart",
			"node",
			"tcp",
			dialFeedbackSignalHandshake,
			true,
			true,
			2*time.Millisecond,
			20*time.Millisecond,
			"",
		)
		buffer.record(
			"smart",
			"node",
			"tcp",
			dialFeedbackSignalFirstByte,
			false,
			true,
			3*time.Millisecond,
			"",
		)
	}

	detailedSequence, detailed := buffer.snapshotDetailed(0)
	legacySequence, legacy := buffer.snapshotLegacy(0)
	if detailedSequence != 900 || legacySequence != detailedSequence {
		t.Fatalf("sequence detailed=%d legacy=%d", detailedSequence, legacySequence)
	}
	if len(detailed) != dialFeedbackCapacity {
		t.Fatalf("detailed events=%d", len(detailed))
	}
	if detailed[0].Sequence != 645 || detailed[len(detailed)-1].Sequence != 900 {
		t.Fatalf("detailed retained range=%d..%d", detailed[0].Sequence, detailed[len(detailed)-1].Sequence)
	}
	if len(legacy) != dialFeedbackCapacity {
		t.Fatalf("legacy events=%d", len(legacy))
	}
	if legacy[0].Sequence != 134 || legacy[len(legacy)-1].Sequence != 899 {
		t.Fatalf("legacy retained range=%d..%d", legacy[0].Sequence, legacy[len(legacy)-1].Sequence)
	}
	if legacy[0].Signal != dialFeedbackSignalHandshake || legacy[0].DurationMs != 20 {
		t.Fatalf("legacy projection=%+v", legacy[0])
	}
	var newestDetailedHandshake *DialFeedbackEvent
	for index := len(detailed) - 1; index >= 0; index-- {
		if detailed[index].Signal == dialFeedbackSignalHandshake {
			newestDetailedHandshake = &detailed[index]
			break
		}
	}
	if newestDetailedHandshake == nil || newestDetailedHandshake.DurationMs != 2 {
		t.Fatalf("detailed handshake=%+v", newestDetailedHandshake)
	}
}

func TestDialFeedbackBufferConcurrentWritersAndSnapshots(t *testing.T) {
	t.Parallel()

	var buffer dialFeedbackBuffer
	var waitGroup sync.WaitGroup
	for writer := 0; writer < 4; writer++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for index := 0; index < 300; index++ {
				legacy := index%2 == 0
				buffer.record(
					"smart",
					"node",
					"tcp",
					dialFeedbackSignalTCP,
					legacy,
					true,
					time.Millisecond,
					"",
				)
			}
		}()
	}
	for reader := 0; reader < 4; reader++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for index := 0; index < 300; index++ {
				buffer.snapshotDetailed(0)
				buffer.snapshotLegacy(0)
			}
		}()
	}
	waitGroup.Wait()

	sequence, detailed := buffer.snapshotDetailed(0)
	legacySequence, legacy := buffer.snapshotLegacy(0)
	if sequence != 1_200 || legacySequence != sequence {
		t.Fatalf("sequence detailed=%d legacy=%d", sequence, legacySequence)
	}
	if len(detailed) != dialFeedbackCapacity || len(legacy) != dialFeedbackCapacity {
		t.Fatalf("bounded lengths detailed=%d legacy=%d", len(detailed), len(legacy))
	}
}

func TestDialFeedbackInstanceIsStableRandomID(t *testing.T) {
	t.Parallel()

	instance := DialFeedbackInstance()
	if len(instance) != 32 {
		t.Fatalf("instance length=%d", len(instance))
	}
	if _, err := hex.DecodeString(instance); err != nil {
		t.Fatalf("instance is not hexadecimal: %v", err)
	}
	if again := DialFeedbackInstance(); again != instance {
		t.Fatalf("instance changed within process: %q != %q", again, instance)
	}
	if fresh := newDialFeedbackInstance(); fresh == instance {
		t.Fatal("fresh random instance unexpectedly matched process instance")
	}
}

func TestDialFeedbackSanitizesEventFields(t *testing.T) {
	t.Parallel()

	var buffer dialFeedbackBuffer
	buffer.record("smart", "node-a", "tcp", dialFeedbackSignalTCP, true, true, 500*time.Microsecond, "unknown")
	buffer.record("smart", "node-b", "udp", dialFeedbackSignalUDP, true, false, time.Millisecond, "secret raw error")
	_, events := buffer.snapshot(0, true)
	if len(events) != 2 {
		t.Fatalf("events=%d", len(events))
	}
	event := events[0]
	if event.Group != "smart" || event.Outbound != "node-a" || event.Network != "tcp" ||
		event.Signal != dialFeedbackSignalTCP || !event.Success {
		t.Fatalf("unexpected event: %+v", event)
	}
	if event.DurationMs != 1 {
		t.Fatalf("durationMs=%d", event.DurationMs)
	}
	if event.Timestamp <= 0 {
		t.Fatalf("timestamp=%d", event.Timestamp)
	}
	if event.ErrorClass != "" {
		t.Fatalf("success errorClass=%q", event.ErrorClass)
	}
	if events[1].ErrorClass != "unknown" {
		t.Fatalf("failure errorClass=%q", events[1].ErrorClass)
	}
	payload, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("marshal events: %v", err)
	}
	if strings.Contains(string(payload), "secret raw error") ||
		strings.Contains(string(payload), "legacy") {
		t.Fatalf("event JSON leaked private/internal data: %s", payload)
	}
}

func TestClassifyDialError(t *testing.T) {
	t.Parallel()

	timeoutError := &net.DNSError{IsTimeout: true}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"success", nil, ""},
		{"deadline", context.DeadlineExceeded, "timeout"},
		{"canceled", context.Canceled, "canceled"},
		{"canceled precedence", errors.Join(context.DeadlineExceeded, context.Canceled), "canceled"},
		{"network timeout", timeoutError, "timeout"},
		{"generic network", errors.New("dial failed"), "network"},
	}
	for _, test := range tests {
		if got := classifyDialError(test.err); got != test.want {
			t.Errorf("%s: got %q want %q", test.name, got, test.want)
		}
	}
}
