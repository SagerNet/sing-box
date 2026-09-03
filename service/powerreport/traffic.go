package powerreport

import "sync/atomic"

type TrafficKind string

const (
	TrafficOutbound     TrafficKind = "outbound"
	TrafficEndpoint     TrafficKind = "endpoint"
	TrafficDNSTransport TrafficKind = "dns"
)

const maxTrafficCounters = 512

type TrafficCounter struct {
	inBytes  atomic.Uint64
	outBytes atomic.Uint64
}

func (c *TrafficCounter) CountIn(n int64) {
	if c == nil || n <= 0 {
		return
	}
	c.inBytes.Add(uint64(n))
}

func (c *TrafficCounter) CountOut(n int64) {
	if c == nil || n <= 0 {
		return
	}
	c.outBytes.Add(uint64(n))
}

func (r *Recorder) TrafficCounter(kind TrafficKind, tag string) *TrafficCounter {
	if r == nil || tag == "" {
		return nil
	}
	key := trafficKey{kind: kind, tag: tag}
	r.trafficAccess.RLock()
	counter := r.traffic[key]
	r.trafficAccess.RUnlock()
	if counter != nil {
		return counter
	}
	r.trafficAccess.Lock()
	defer r.trafficAccess.Unlock()
	counter = r.traffic[key]
	if counter != nil {
		return counter
	}
	if len(r.traffic) >= maxTrafficCounters {
		return nil
	}
	counter = new(TrafficCounter)
	r.traffic[key] = counter
	return counter
}

func (r *Recorder) snapshotTrafficLocked() map[trafficKey]trafficBytes {
	r.trafficAccess.RLock()
	defer r.trafficAccess.RUnlock()
	if len(r.traffic) == 0 {
		return nil
	}
	snapshot := make(map[trafficKey]trafficBytes, len(r.traffic))
	for key, counter := range r.traffic {
		snapshot[key] = trafficBytes{
			inBytes:  counter.inBytes.Load(),
			outBytes: counter.outBytes.Load(),
		}
	}
	return snapshot
}
