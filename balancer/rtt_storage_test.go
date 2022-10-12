package balancer

import (
	"math"
	"reflect"
	"testing"
	"time"
)

func TestRTTStorage(t *testing.T) {
	rtts := []int64{60, 140, 60, 140, 60, 60, 140, 60, 140}
	s := newRTTStorage(4, time.Hour)
	for _, rtt := range rtts {
		s.Put(time.Duration(rtt))
	}
	rttFailed := time.Duration(math.MaxInt64)
	want := RTTStats{
		All:       4,
		Fail:      0,
		Deviation: 40,
		Average:   100,
		Max:       140,
		Min:       60,
	}
	got := s.Get()
	if !reflect.DeepEqual(want, got) {
		t.Errorf("want: %v, got: %v", want, got)
	}
	s.Put(rttFailed)
	s.Put(rttFailed)
	want.Fail = 2
	got = s.Get()
	if !reflect.DeepEqual(want, got) {
		t.Errorf("failed half-failures test, want: %v, got: %v", want, got)
	}
	s.Put(rttFailed)
	s.Put(rttFailed)
	want = RTTStats{
		All:       4,
		Fail:      4,
		Deviation: rttFailed,
		Average:   rttFailed,
		Max:       rttFailed,
		Min:       rttFailed,
	}
	got = s.Get()
	if !reflect.DeepEqual(want, got) {
		t.Errorf("failed all-failures test, want: %v, got: %v", want, got)
	}
}

func TestHealthPingResultsIgnoreOutdated(t *testing.T) {
	rtts := []int64{60, 140, 60, 140}
	s := newRTTStorage(4, time.Duration(10)*time.Millisecond)
	for i, rtt := range rtts {
		if i == 2 {
			// wait for previous 2 outdated
			time.Sleep(time.Duration(10) * time.Millisecond)
		}
		s.Put(time.Duration(rtt))
	}
	s.Get()
	want := RTTStats{
		All:       2,
		Fail:      0,
		Deviation: 40,
		Average:   100,
		Max:       140,
		Min:       60,
	}
	got := s.Get()
	if !reflect.DeepEqual(want, got) {
		t.Errorf("failed 'half-outdated' test, want: %v, got: %v", want, got)
	}
	// wait for all outdated
	time.Sleep(time.Duration(10) * time.Millisecond)
	want = RTTStats{
		All:       0,
		Fail:      0,
		Deviation: rttUntested,
		Average:   rttUntested,
		Max:       rttUntested,
		Min:       rttUntested,
	}
	got = s.Get()
	if !reflect.DeepEqual(want, got) {
		t.Errorf("failed 'outdated / not-tested' test, want: %v, got: %v", want, got)
	}

	s.Put(time.Duration(60))
	want = RTTStats{
		All:  1,
		Fail: 0,
		// 1 sample, std=0.5rtt
		Deviation: 30,
		Average:   60,
		Max:       60,
		Min:       60,
	}
	got = s.Get()
	if !reflect.DeepEqual(want, got) {
		t.Errorf("want: %v, got: %v", want, got)
	}
}
