package congestion

import (
	"fmt"
	"time"

	"github.com/sagernet/quic-go/congestion"
	"github.com/sagernet/quic-go/logging"
)

const (
	maxBurstPackets            = 3
	renoBeta                   = 0.7 // Reno backoff factor.
	minCongestionWindowPackets = 2
	initialCongestionWindow    = 32
)

const (
	InvalidPacketNumber        congestion.PacketNumber = -1
	MaxCongestionWindowPackets                         = 20000
	MaxByteCount                                       = congestion.ByteCount(1<<62 - 1)
)

type cubicSender struct {
	hybridSlowStart HybridSlowStart
	rttStats        congestion.RTTStatsProvider
	cubic           *Cubic
	pacer           *pacer
	clock           Clock

	reno bool

	// Track the largest packet that has been sent.
	largestSentPacketNumber congestion.PacketNumber

	// Track the largest packet that has been acked.
	largestAckedPacketNumber congestion.PacketNumber

	// Track the largest packet number outstanding when a CWND cutback occurs.
	largestSentAtLastCutback congestion.PacketNumber

	// Whether the last loss event caused us to exit slowstart.
	// Used for stats collection of slowstartPacketsLost
	lastCutbackExitedSlowstart bool

	// Congestion window in bytes.
	congestionWindow congestion.ByteCount

	// Slow start congestion window in bytes, aka ssthresh.
	slowStartThreshold congestion.ByteCount

	// ACK counter for the Reno implementation.
	numAckedPackets uint64

	initialCongestionWindow    congestion.ByteCount
	initialMaxCongestionWindow congestion.ByteCount

	maxDatagramSize congestion.ByteCount

	lastState logging.CongestionState
	tracer    logging.ConnectionTracer
}

var _ congestion.CongestionControl = &cubicSender{}

// NewCubicSender makes a new cubic sender
func NewCubicSender(
	clock Clock,
	initialMaxDatagramSize congestion.ByteCount,
	reno bool,
	tracer logging.ConnectionTracer,
) *cubicSender {
	return newCubicSender(
		clock,
		reno,
		initialMaxDatagramSize,
		initialCongestionWindow*initialMaxDatagramSize,
		MaxCongestionWindowPackets*initialMaxDatagramSize,
		tracer,
	)
}

func newCubicSender(
	clock Clock,
	reno bool,
	initialMaxDatagramSize,
	initialCongestionWindow,
	initialMaxCongestionWindow congestion.ByteCount,
	tracer logging.ConnectionTracer,
) *cubicSender {
	c := &cubicSender{
		largestSentPacketNumber:    InvalidPacketNumber,
		largestAckedPacketNumber:   InvalidPacketNumber,
		largestSentAtLastCutback:   InvalidPacketNumber,
		initialCongestionWindow:    initialCongestionWindow,
		initialMaxCongestionWindow: initialMaxCongestionWindow,
		congestionWindow:           initialCongestionWindow,
		slowStartThreshold:         MaxByteCount,
		cubic:                      NewCubic(clock),
		clock:                      clock,
		reno:                       reno,
		tracer:                     tracer,
		maxDatagramSize:            initialMaxDatagramSize,
	}
	c.pacer = newPacer(c.BandwidthEstimate)
	if c.tracer != nil {
		c.lastState = logging.CongestionStateSlowStart
		c.tracer.UpdatedCongestionState(logging.CongestionStateSlowStart)
	}
	return c
}

func (c *cubicSender) SetRTTStatsProvider(provider congestion.RTTStatsProvider) {
	c.rttStats = provider
}

// TimeUntilSend returns when the next packet should be sent.
func (c *cubicSender) TimeUntilSend(_ congestion.ByteCount) time.Time {
	return c.pacer.TimeUntilSend()
}

func (c *cubicSender) HasPacingBudget(now time.Time) bool {
	return c.pacer.Budget(now) >= c.maxDatagramSize
}

func (c *cubicSender) maxCongestionWindow() congestion.ByteCount {
	return c.maxDatagramSize * MaxCongestionWindowPackets
}

func (c *cubicSender) minCongestionWindow() congestion.ByteCount {
	return c.maxDatagramSize * minCongestionWindowPackets
}

func (c *cubicSender) OnPacketSent(
	sentTime time.Time,
	_ congestion.ByteCount,
	packetNumber congestion.PacketNumber,
	bytes congestion.ByteCount,
	isRetransmittable bool,
) {
	c.pacer.SentPacket(sentTime, bytes)
	if !isRetransmittable {
		return
	}
	c.largestSentPacketNumber = packetNumber
	c.hybridSlowStart.OnPacketSent(packetNumber)
}

func (c *cubicSender) CanSend(bytesInFlight congestion.ByteCount) bool {
	return bytesInFlight < c.GetCongestionWindow()
}

func (c *cubicSender) InRecovery() bool {
	return c.largestAckedPacketNumber != InvalidPacketNumber && c.largestAckedPacketNumber <= c.largestSentAtLastCutback
}

func (c *cubicSender) InSlowStart() bool {
	return c.GetCongestionWindow() < c.slowStartThreshold
}

func (c *cubicSender) GetCongestionWindow() congestion.ByteCount {
	return c.congestionWindow
}

func (c *cubicSender) MaybeExitSlowStart() {
	if c.InSlowStart() &&
		c.hybridSlowStart.ShouldExitSlowStart(c.rttStats.LatestRTT(), c.rttStats.MinRTT(), c.GetCongestionWindow()/c.maxDatagramSize) {
		// exit slow start
		c.slowStartThreshold = c.congestionWindow
		c.maybeTraceStateChange(logging.CongestionStateCongestionAvoidance)
	}
}

func (c *cubicSender) OnPacketAcked(
	ackedPacketNumber congestion.PacketNumber,
	ackedBytes congestion.ByteCount,
	priorInFlight congestion.ByteCount,
	eventTime time.Time,
) {
	c.largestAckedPacketNumber = Max(ackedPacketNumber, c.largestAckedPacketNumber)
	if c.InRecovery() {
		return
	}
	c.maybeIncreaseCwnd(ackedPacketNumber, ackedBytes, priorInFlight, eventTime)
	if c.InSlowStart() {
		c.hybridSlowStart.OnPacketAcked(ackedPacketNumber)
	}
}

func (c *cubicSender) OnPacketLost(packetNumber congestion.PacketNumber, lostBytes, priorInFlight congestion.ByteCount) {
	// TCP NewReno (RFC6582) says that once a loss occurs, any losses in packets
	// already sent should be treated as a single loss event, since it's expected.
	if packetNumber <= c.largestSentAtLastCutback {
		return
	}
	c.lastCutbackExitedSlowstart = c.InSlowStart()
	c.maybeTraceStateChange(logging.CongestionStateRecovery)

	if c.reno {
		c.congestionWindow = congestion.ByteCount(float64(c.congestionWindow) * renoBeta)
	} else {
		c.congestionWindow = c.cubic.CongestionWindowAfterPacketLoss(c.congestionWindow)
	}
	if minCwnd := c.minCongestionWindow(); c.congestionWindow < minCwnd {
		c.congestionWindow = minCwnd
	}
	c.slowStartThreshold = c.congestionWindow
	c.largestSentAtLastCutback = c.largestSentPacketNumber
	// reset packet count from congestion avoidance mode. We start
	// counting again when we're out of recovery.
	c.numAckedPackets = 0
}

// Called when we receive an ack. Normal TCP tracks how many packets one ack
// represents, but quic has a separate ack for each packet.
func (c *cubicSender) maybeIncreaseCwnd(
	_ congestion.PacketNumber,
	ackedBytes congestion.ByteCount,
	priorInFlight congestion.ByteCount,
	eventTime time.Time,
) {
	// Do not increase the congestion window unless the sender is close to using
	// the current window.
	if !c.isCwndLimited(priorInFlight) {
		c.cubic.OnApplicationLimited()
		c.maybeTraceStateChange(logging.CongestionStateApplicationLimited)
		return
	}
	if c.congestionWindow >= c.maxCongestionWindow() {
		return
	}
	if c.InSlowStart() {
		// TCP slow start, exponential growth, increase by one for each ACK.
		c.congestionWindow += c.maxDatagramSize
		c.maybeTraceStateChange(logging.CongestionStateSlowStart)
		return
	}
	// Congestion avoidance
	c.maybeTraceStateChange(logging.CongestionStateCongestionAvoidance)
	if c.reno {
		// Classic Reno congestion avoidance.
		c.numAckedPackets++
		if c.numAckedPackets >= uint64(c.congestionWindow/c.maxDatagramSize) {
			c.congestionWindow += c.maxDatagramSize
			c.numAckedPackets = 0
		}
	} else {
		c.congestionWindow = Min(c.maxCongestionWindow(), c.cubic.CongestionWindowAfterAck(ackedBytes, c.congestionWindow, c.rttStats.MinRTT(), eventTime))
	}
}

func (c *cubicSender) isCwndLimited(bytesInFlight congestion.ByteCount) bool {
	congestionWindow := c.GetCongestionWindow()
	if bytesInFlight >= congestionWindow {
		return true
	}
	availableBytes := congestionWindow - bytesInFlight
	slowStartLimited := c.InSlowStart() && bytesInFlight > congestionWindow/2
	return slowStartLimited || availableBytes <= maxBurstPackets*c.maxDatagramSize
}

// BandwidthEstimate returns the current bandwidth estimate
func (c *cubicSender) BandwidthEstimate() Bandwidth {
	if c.rttStats == nil {
		return infBandwidth
	}
	srtt := c.rttStats.SmoothedRTT()
	if srtt == 0 {
		// If we haven't measured an rtt, the bandwidth estimate is unknown.
		return infBandwidth
	}
	return BandwidthFromDelta(c.GetCongestionWindow(), srtt)
}

// OnRetransmissionTimeout is called on an retransmission timeout
func (c *cubicSender) OnRetransmissionTimeout(packetsRetransmitted bool) {
	c.largestSentAtLastCutback = InvalidPacketNumber
	if !packetsRetransmitted {
		return
	}
	c.hybridSlowStart.Restart()
	c.cubic.Reset()
	c.slowStartThreshold = c.congestionWindow / 2
	c.congestionWindow = c.minCongestionWindow()
}

// OnConnectionMigration is called when the connection is migrated (?)
func (c *cubicSender) OnConnectionMigration() {
	c.hybridSlowStart.Restart()
	c.largestSentPacketNumber = InvalidPacketNumber
	c.largestAckedPacketNumber = InvalidPacketNumber
	c.largestSentAtLastCutback = InvalidPacketNumber
	c.lastCutbackExitedSlowstart = false
	c.cubic.Reset()
	c.numAckedPackets = 0
	c.congestionWindow = c.initialCongestionWindow
	c.slowStartThreshold = c.initialMaxCongestionWindow
}

func (c *cubicSender) maybeTraceStateChange(new logging.CongestionState) {
	if c.tracer == nil || new == c.lastState {
		return
	}
	c.tracer.UpdatedCongestionState(new)
	c.lastState = new
}

func (c *cubicSender) SetMaxDatagramSize(s congestion.ByteCount) {
	if s < c.maxDatagramSize {
		panic(fmt.Sprintf("congestion BUG: decreased max datagram size from %d to %d", c.maxDatagramSize, s))
	}
	cwndIsMinCwnd := c.congestionWindow == c.minCongestionWindow()
	c.maxDatagramSize = s
	if cwndIsMinCwnd {
		c.congestionWindow = c.minCongestionWindow()
	}
	c.pacer.SetMaxDatagramSize(s)
}
