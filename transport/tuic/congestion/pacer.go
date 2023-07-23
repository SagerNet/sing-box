package congestion

import (
	"math"
	"time"

	"github.com/sagernet/quic-go/congestion"
)

const (
	initialMaxDatagramSize = congestion.ByteCount(1252)
	MinPacingDelay         = time.Millisecond
	TimerGranularity       = time.Millisecond
	maxBurstSizePackets    = 10
)

// The pacer implements a token bucket pacing algorithm.
type pacer struct {
	budgetAtLastSent     congestion.ByteCount
	maxDatagramSize      congestion.ByteCount
	lastSentTime         time.Time
	getAdjustedBandwidth func() uint64 // in bytes/s
}

func newPacer(getBandwidth func() Bandwidth) *pacer {
	p := &pacer{
		maxDatagramSize: initialMaxDatagramSize,
		getAdjustedBandwidth: func() uint64 {
			// Bandwidth is in bits/s. We need the value in bytes/s.
			bw := uint64(getBandwidth() / BytesPerSecond)
			// Use a slightly higher value than the actual measured bandwidth.
			// RTT variations then won't result in under-utilization of the congestion window.
			// Ultimately, this will  result in sending packets as acknowledgments are received rather than when timers fire,
			// provided the congestion window is fully utilized and acknowledgments arrive at regular intervals.
			return bw * 5 / 4
		},
	}
	p.budgetAtLastSent = p.maxBurstSize()
	return p
}

func (p *pacer) SentPacket(sendTime time.Time, size congestion.ByteCount) {
	budget := p.Budget(sendTime)
	if size > budget {
		p.budgetAtLastSent = 0
	} else {
		p.budgetAtLastSent = budget - size
	}
	p.lastSentTime = sendTime
}

func (p *pacer) Budget(now time.Time) congestion.ByteCount {
	if p.lastSentTime.IsZero() {
		return p.maxBurstSize()
	}
	budget := p.budgetAtLastSent + (congestion.ByteCount(p.getAdjustedBandwidth())*congestion.ByteCount(now.Sub(p.lastSentTime).Nanoseconds()))/1e9
	return Min(p.maxBurstSize(), budget)
}

func (p *pacer) maxBurstSize() congestion.ByteCount {
	return Max(
		congestion.ByteCount(uint64((MinPacingDelay+TimerGranularity).Nanoseconds())*p.getAdjustedBandwidth())/1e9,
		maxBurstSizePackets*p.maxDatagramSize,
	)
}

// TimeUntilSend returns when the next packet should be sent.
// It returns the zero value of time.Time if a packet can be sent immediately.
func (p *pacer) TimeUntilSend() time.Time {
	if p.budgetAtLastSent >= p.maxDatagramSize {
		return time.Time{}
	}
	return p.lastSentTime.Add(Max(
		MinPacingDelay,
		time.Duration(math.Ceil(float64(p.maxDatagramSize-p.budgetAtLastSent)*1e9/float64(p.getAdjustedBandwidth())))*time.Nanosecond,
	))
}

func (p *pacer) SetMaxDatagramSize(s congestion.ByteCount) {
	p.maxDatagramSize = s
}
