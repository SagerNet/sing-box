package balancer

import (
	"math"
	"time"
)

const (
	rttFailed = time.Duration(math.MaxInt64 - iota)
	rttUntested
	rttUnqualified
)

// HealthCheckStats is the statistics of HealthPingRTTS
type HealthCheckStats struct {
	All       int
	Fail      int
	Deviation time.Duration
	Average   time.Duration
	Max       time.Duration
	Min       time.Duration

	Weighted time.Duration
}

// HealthCheckRTTS holds ping rtts for health Checker
type HealthCheckRTTS struct {
	idx      int
	cap      int
	validity time.Duration
	rtts     []*pingRTT

	lastUpdateAt time.Time
	stats        *HealthCheckStats
}

type pingRTT struct {
	time  time.Time
	value time.Duration
}

// NewHealthPingResult returns a *HealthPingResult with specified capacity
func NewHealthPingResult(cap int, validity time.Duration) *HealthCheckRTTS {
	return &HealthCheckRTTS{cap: cap, validity: validity}
}

// Get gets statistics of the HealthPingRTTS
func (h *HealthCheckRTTS) Get() HealthCheckStats {
	return h.getStatistics()
}

// GetWithCache get statistics and write cache for next call
// Make sure use Mutex.Lock() before calling it, RWMutex.RLock()
// is not an option since it writes cache
func (h *HealthCheckRTTS) GetWithCache() HealthCheckStats {
	lastPutAt := h.rtts[h.idx].time
	now := time.Now()
	if h.stats == nil || h.lastUpdateAt.Before(lastPutAt) || h.findOutdated(now) >= 0 {
		if h.stats == nil {
			h.stats = &HealthCheckStats{}
		}
		*h.stats = h.getStatistics()
		h.lastUpdateAt = now
	}
	return *h.stats
}

// Put puts a new rtt to the HealthPingResult
func (h *HealthCheckRTTS) Put(d time.Duration) {
	if h.rtts == nil {
		h.rtts = make([]*pingRTT, h.cap)
		for i := 0; i < h.cap; i++ {
			h.rtts[i] = &pingRTT{}
		}
		h.idx = -1
	}
	h.idx = h.calcIndex(1)
	now := time.Now()
	h.rtts[h.idx].time = now
	h.rtts[h.idx].value = d
}

func (h *HealthCheckRTTS) calcIndex(step int) int {
	idx := h.idx
	idx += step
	if idx >= h.cap {
		idx %= h.cap
	}
	return idx
}

func (h *HealthCheckRTTS) getStatistics() HealthCheckStats {
	stats := HealthCheckStats{}
	stats.Fail = 0
	stats.Max = 0
	stats.Min = rttFailed
	sum := time.Duration(0)
	cnt := 0
	validRTTs := make([]time.Duration, 0, h.cap)
	for _, rtt := range h.rtts {
		switch {
		case rtt.value == 0 || time.Since(rtt.time) > h.validity:
			continue
		case rtt.value == rttFailed:
			stats.Fail++
			continue
		}
		cnt++
		sum += rtt.value
		validRTTs = append(validRTTs, rtt.value)
		if stats.Max < rtt.value {
			stats.Max = rtt.value
		}
		if stats.Min > rtt.value {
			stats.Min = rtt.value
		}
	}
	stats.All = cnt + stats.Fail
	if cnt == 0 {
		stats.Min = 0
		return healthPingStatsUntested
	}
	stats.Average = time.Duration(int(sum) / cnt)
	switch {
	case stats.All == 0:
		return healthPingStatsUntested
	case stats.Fail == stats.All:
		return HealthCheckStats{
			All:       stats.All,
			Fail:      stats.Fail,
			Deviation: rttFailed,
			Average:   rttFailed,
			Max:       rttFailed,
			Min:       rttFailed,
		}
	}
	var std float64
	if cnt < 2 {
		// no enough data for standard deviation, we assume it's half of the average rtt
		// if we don't do this, standard deviation of 1 round tested nodes is 0, will always
		// selected before 2 or more rounds tested nodes
		std = float64(stats.Average / 2)
	} else {
		variance := float64(0)
		for _, rtt := range validRTTs {
			variance += math.Pow(float64(rtt-stats.Average), 2)
		}
		std = math.Sqrt(variance / float64(cnt))
	}
	stats.Deviation = time.Duration(std)
	return stats
}

func (h *HealthCheckRTTS) findOutdated(now time.Time) int {
	for i := h.cap - 1; i < 2*h.cap; i++ {
		// from oldest to latest
		idx := h.calcIndex(i)
		validity := h.rtts[idx].time.Add(h.validity)
		if h.lastUpdateAt.After(validity) {
			return idx
		}
		if validity.Before(now) {
			return idx
		}
	}
	return -1
}
