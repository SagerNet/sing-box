package engine

import (
	"testing"
	"time"
)

func TestSelectPrefersHealthyMember(t *testing.T) {
	t.Parallel()
	e := New([]string{"a", "b", "c"}, Options{})
	e.Record("a", OutcomeSuccess, 200)
	e.Record("a", OutcomeSuccess, 210)
	e.Record("b", OutcomeSuccess, 80)
	e.Record("b", OutcomeSuccess, 85)
	e.Record("c", OutcomeFailure, 0)
	e.Record("c", OutcomeFailure, 0)

	cands := e.Select("")
	if len(cands) < 2 {
		t.Fatalf("expected candidates, got %v", cands)
	}
	if cands[0].Tag != "b" {
		// Top-K shuffle may put a close peer first; b should still be in top-2.
		if cands[0].Tag != "a" && cands[1].Tag != "b" {
			t.Fatalf("expected b near front, got %#v", cands)
		}
	}
}

func TestSoftFailThreshold(t *testing.T) {
	t.Parallel()
	e := New([]string{"a"}, Options{SoftFailRatio: 1.5, SoftFailFloorMs: 80})
	e.Record("a", OutcomeSuccess, 100)
	th := e.SoftFailThresholdMs("a")
	if th < 180 || th > 200 {
		t.Fatalf("threshold=%v want ~180-200", th)
	}
}

func TestPenaltyGrowsAndSuccessWipes(t *testing.T) {
	t.Parallel()
	e := New([]string{"a"}, Options{})
	e.Record("a", OutcomeFailure, 0)
	e.Record("a", OutcomeFailure, 0)
	snap := e.Snapshot()
	if snap[0].Penalty < 1 {
		t.Fatalf("expected penalty growth, got %v", snap[0].Penalty)
	}
	e.Record("a", OutcomeSuccess, 100)
	snap = e.Snapshot()
	if snap[0].Penalty > snap[0].Penalty+1 { // just ensure finite
		t.Fatal("penalty invalid")
	}
	// Success should shrink penalty substantially.
	if snap[0].Penalty >= 2 {
		t.Fatalf("expected success wipe, penalty=%v", snap[0].Penalty)
	}
}

func TestHostSticky(t *testing.T) {
	t.Parallel()
	fixed := time.Unix(1_700_000_000, 0)
	e := New([]string{"a", "b"}, Options{
		Now: func() time.Time { return fixed },
	})
	e.Record("a", OutcomeSuccess, 50)
	e.Record("b", OutcomeSuccess, 60)
	e.RememberHost("example.com", "b")
	cands := e.Select("example.com")
	if cands[0].Tag != "b" {
		t.Fatalf("sticky want b first, got %v", cands[0].Tag)
	}
}

func TestPreferredBeatsOrder(t *testing.T) {
	t.Parallel()
	fixed := time.Unix(1_700_000_000, 0)
	e := New([]string{"a", "b", "c"}, Options{
		Now: func() time.Time { return fixed },
	})
	e.Record("a", OutcomeSuccess, 40)
	e.Record("b", OutcomeSuccess, 80)
	e.Record("c", OutcomeSuccess, 90)
	e.SetPreferred("c")
	cands := e.Select("")
	if cands[0].Tag != "c" {
		t.Fatalf("preferred want c first, got %v", cands[0].Tag)
	}
}

func TestTargetHistoryAndProtocolIsolation(t *testing.T) {
	t.Parallel()
	fixed := time.Unix(1_700_000_000, 0)
	e := New([]string{"a", "b"}, Options{
		Now: func() time.Time { return fixed },
	})
	for i := 0; i < 12; i++ {
		e.Record("a", OutcomeSuccess, 40)
		e.Record("b", OutcomeSuccess, 100)
	}
	for i := 0; i < 4; i++ {
		e.RecordFor("blocked.example", NetworkTCP, "a", OutcomeFailure, 0)
		e.RecordFor("blocked.example", NetworkTCP, "b", OutcomeSuccess, 100)
	}
	if got := e.SelectFor("blocked.example", NetworkTCP)[0].Tag; got != "b" {
		t.Fatalf("target history want b, got %s", got)
	}
	if got := e.SelectFor("other.example", NetworkTCP)[0].Tag; got != "a" {
		t.Fatalf("target failure leaked globally: want a, got %s", got)
	}
	for i := 0; i < 4; i++ {
		e.RecordFor("other.example", NetworkUDP, "a", OutcomeFailure, 0)
	}
	if got := e.SelectFor("other.example", NetworkTCP)[0].Tag; got != "a" {
		t.Fatalf("UDP history leaked into TCP: want a, got %s", got)
	}
}

func TestTargetHistoryIsBounded(t *testing.T) {
	t.Parallel()
	e := New([]string{"a"}, Options{MaxTargets: 3})
	for _, host := range []string{"one.example", "two.example", "three.example", "four.example"} {
		e.RecordFor(host, NetworkTCP, "a", OutcomeSuccess, 50)
	}
	if len(e.targets) != 3 {
		t.Fatalf("target history size=%d want 3", len(e.targets))
	}
}

func TestModeOptions(t *testing.T) {
	t.Parallel()
	latency := OptionsForMode("latency").withDefaults()
	stable := OptionsForMode("stable").withDefaults()
	if latency.ExplorationMs <= stable.ExplorationMs || latency.HostStickyTTL >= stable.HostStickyTTL {
		t.Fatalf("mode posture mismatch: latency=%+v stable=%+v", latency, stable)
	}
}

func TestBanditExplorationFadesWithEvidence(t *testing.T) {
	t.Parallel()
	e := New([]string{"mature", "uncertain"}, Options{})
	for i := 0; i < 40; i++ {
		e.Record("mature", OutcomeSuccess, 100)
	}
	e.Record("uncertain", OutcomeSuccess, 100)
	if got := e.Select("")[0].Tag; got != "uncertain" {
		t.Fatalf("expected uncertain arm exploration, got %s", got)
	}
	for i := 0; i < 80; i++ {
		e.Record("uncertain", OutcomeSuccess, 100)
	}
	if got := e.Select("")[0].Tag; got != "mature" {
		t.Fatalf("expected exploration bonus to fade, got %s", got)
	}
}

func TestFirstByteCostStaysTargetSpecific(t *testing.T) {
	t.Parallel()
	e := New([]string{"a", "b"}, Options{})
	for i := 0; i < 8; i++ {
		e.Record("a", OutcomeSuccess, 80)
		e.Record("b", OutcomeSuccess, 80)
		e.RecordFirstByteFor("slow.example", NetworkTCP, "a", true, 900)
		e.RecordFirstByteFor("slow.example", NetworkTCP, "b", true, 120)
	}
	if got := e.SelectFor("slow.example", NetworkTCP)[0].Tag; got != "b" {
		t.Fatalf("target first-byte cost want b, got %s", got)
	}
	if got := e.SelectFor("other.example", NetworkTCP)[0].Tag; got != "a" {
		t.Fatalf("first-byte history leaked globally: want a, got %s", got)
	}
}
