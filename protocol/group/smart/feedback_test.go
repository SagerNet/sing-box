package smart

import (
	"context"
	"encoding/hex"
	"errors"
	"net"
	"testing"
	"time"
)

func TestDialFeedbackBufferIsBoundedAndIncremental(t *testing.T) {
	t.Parallel()

	var buffer dialFeedbackBuffer
	for index := 0; index < dialFeedbackCapacity+4; index++ {
		buffer.record("smart", "node", "tcp", index%2 == 0, time.Duration(index)*time.Millisecond, "unknown")
	}

	sequence, events := buffer.snapshot(0)
	if sequence != dialFeedbackCapacity+4 {
		t.Fatalf("sequence=%d", sequence)
	}
	if len(events) != dialFeedbackCapacity {
		t.Fatalf("events=%d", len(events))
	}
	if events[0].Sequence != 5 || events[len(events)-1].Sequence != sequence {
		t.Fatalf("unexpected retained range %d..%d", events[0].Sequence, events[len(events)-1].Sequence)
	}

	_, incremental := buffer.snapshot(sequence - 2)
	if len(incremental) != 2 || incremental[0].Sequence != sequence-1 || incremental[1].Sequence != sequence {
		t.Fatalf("unexpected incremental events: %+v", incremental)
	}

	current, future := buffer.snapshot(sequence + 10)
	if current != sequence || len(future) != 0 {
		t.Fatalf("future cursor response sequence=%d events=%d", current, len(future))
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
	buffer.record("smart", "node-a", "tcp", true, 500*time.Microsecond, "unknown")
	buffer.record("smart", "node-b", "udp", false, time.Millisecond, "secret raw error")
	_, events := buffer.snapshot(0)
	if len(events) != 2 {
		t.Fatalf("events=%d", len(events))
	}
	event := events[0]
	if event.Group != "smart" || event.Outbound != "node-a" || event.Network != "tcp" || !event.Success {
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
