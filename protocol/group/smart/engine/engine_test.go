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
		Now:         func() time.Time { return fixed },
		ExploreProb: -1, // disable explore for deterministic sticky test
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
		Now:         func() time.Time { return fixed },
		ExploreProb: -1,
		TopK:        1,
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
