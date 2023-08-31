package congestion

import (
	"math"
	"time"

	"github.com/sagernet/quic-go/congestion"
)

const (
	maxBurstPackets = 10
	minPacingDelay  = time.Millisecond
)

// The pacer implements a token bucket pacing algorithm.
type pacer struct {
	budgetAtLastSent congestion.ByteCount
	maxDatagramSize  congestion.ByteCount
	lastSentTime     time.Time
	getBandwidth     func() congestion.ByteCount // in bytes/s
}

func newPacer(getBandwidth func() congestion.ByteCount) *pacer {
	p := &pacer{
		budgetAtLastSent: maxBurstPackets * initMaxDatagramSize,
		maxDatagramSize:  initMaxDatagramSize,
		getBandwidth:     getBandwidth,
	}
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
	budget := p.budgetAtLastSent + (p.getBandwidth()*congestion.ByteCount(now.Sub(p.lastSentTime).Nanoseconds()))/1e9
	return minByteCount(p.maxBurstSize(), budget)
}

func (p *pacer) maxBurstSize() congestion.ByteCount {
	return maxByteCount(
		congestion.ByteCount((minPacingDelay+time.Millisecond).Nanoseconds())*p.getBandwidth()/1e9,
		maxBurstPackets*p.maxDatagramSize,
	)
}

// TimeUntilSend returns when the next packet should be sent.
// It returns the zero value of time.Time if a packet can be sent immediately.
func (p *pacer) TimeUntilSend() time.Time {
	if p.budgetAtLastSent >= p.maxDatagramSize {
		return time.Time{}
	}
	return p.lastSentTime.Add(maxDuration(
		minPacingDelay,
		time.Duration(math.Ceil(float64(p.maxDatagramSize-p.budgetAtLastSent)*1e9/
			float64(p.getBandwidth())))*time.Nanosecond,
	))
}

func (p *pacer) SetMaxDatagramSize(s congestion.ByteCount) {
	p.maxDatagramSize = s
}

func maxByteCount(a, b congestion.ByteCount) congestion.ByteCount {
	if a < b {
		return b
	}
	return a
}

func minByteCount(a, b congestion.ByteCount) congestion.ByteCount {
	if a < b {
		return a
	}
	return b
}
