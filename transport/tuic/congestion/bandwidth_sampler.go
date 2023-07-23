package congestion

import (
	"math"
	"time"

	"github.com/sagernet/quic-go/congestion"
)

var InfiniteBandwidth = Bandwidth(math.MaxUint64)

// SendTimeState is a subset of ConnectionStateOnSentPacket which is returned
// to the caller when the packet is acked or lost.
type SendTimeState struct {
	// Whether other states in this object is valid.
	isValid bool
	// Whether the sender is app limited at the time the packet was sent.
	// App limited bandwidth sample might be artificially low because the sender
	// did not have enough data to send in order to saturate the link.
	isAppLimited bool
	// Total number of sent bytes at the time the packet was sent.
	// Includes the packet itself.
	totalBytesSent congestion.ByteCount
	// Total number of acked bytes at the time the packet was sent.
	totalBytesAcked congestion.ByteCount
	// Total number of lost bytes at the time the packet was sent.
	totalBytesLost congestion.ByteCount
}

// ConnectionStateOnSentPacket represents the information about a sent packet
// and the state of the connection at the moment the packet was sent,
// specifically the information about the most recently acknowledged packet at
// that moment.
type ConnectionStateOnSentPacket struct {
	packetNumber congestion.PacketNumber
	// Time at which the packet is sent.
	sendTime time.Time
	// Size of the packet.
	size congestion.ByteCount
	// The value of |totalBytesSentAtLastAckedPacket| at the time the
	// packet was sent.
	totalBytesSentAtLastAckedPacket congestion.ByteCount
	// The value of |lastAckedPacketSentTime| at the time the packet was
	// sent.
	lastAckedPacketSentTime time.Time
	// The value of |lastAckedPacketAckTime| at the time the packet was
	// sent.
	lastAckedPacketAckTime time.Time
	// Send time states that are returned to the congestion controller when the
	// packet is acked or lost.
	sendTimeState SendTimeState
}

// BandwidthSample
type BandwidthSample struct {
	// The bandwidth at that particular sample. Zero if no valid bandwidth sample
	// is available.
	bandwidth Bandwidth
	// The RTT measurement at this particular sample.  Zero if no RTT sample is
	// available.  Does not correct for delayed ack time.
	rtt time.Duration
	// States captured when the packet was sent.
	stateAtSend SendTimeState
}

func NewBandwidthSample() *BandwidthSample {
	return &BandwidthSample{
		// FIXME: the default value of original code is zero.
		rtt: InfiniteRTT,
	}
}

// BandwidthSampler keeps track of sent and acknowledged packets and outputs a
// bandwidth sample for every packet acknowledged. The samples are taken for
// individual packets, and are not filtered; the consumer has to filter the
// bandwidth samples itself. In certain cases, the sampler will locally severely
// underestimate the bandwidth, hence a maximum filter with a size of at least
// one RTT is recommended.
//
// This class bases its samples on the slope of two curves: the number of bytes
// sent over time, and the number of bytes acknowledged as received over time.
// It produces a sample of both slopes for every packet that gets acknowledged,
// based on a slope between two points on each of the corresponding curves. Note
// that due to the packet loss, the number of bytes on each curve might get
// further and further away from each other, meaning that it is not feasible to
// compare byte values coming from different curves with each other.
//
// The obvious points for measuring slope sample are the ones corresponding to
// the packet that was just acknowledged. Let us denote them as S_1 (point at
// which the current packet was sent) and A_1 (point at which the current packet
// was acknowledged). However, taking a slope requires two points on each line,
// so estimating bandwidth requires picking a packet in the past with respect to
// which the slope is measured.
//
// For that purpose, BandwidthSampler always keeps track of the most recently
// acknowledged packet, and records it together with every outgoing packet.
// When a packet gets acknowledged (A_1), it has not only information about when
// it itself was sent (S_1), but also the information about the latest
// acknowledged packet right before it was sent (S_0 and A_0).
//
// Based on that data, send and ack rate are estimated as:
//
//	send_rate = (bytes(S_1) - bytes(S_0)) / (time(S_1) - time(S_0))
//	ack_rate = (bytes(A_1) - bytes(A_0)) / (time(A_1) - time(A_0))
//
// Here, the ack rate is intuitively the rate we want to treat as bandwidth.
// However, in certain cases (e.g. ack compression) the ack rate at a point may
// end up higher than the rate at which the data was originally sent, which is
// not indicative of the real bandwidth. Hence, we use the send rate as an upper
// bound, and the sample value is
//
//	rate_sample = min(send_rate, ack_rate)
//
// An important edge case handled by the sampler is tracking the app-limited
// samples. There are multiple meaning of "app-limited" used interchangeably,
// hence it is important to understand and to be able to distinguish between
// them.
//
// Meaning 1: connection state. The connection is said to be app-limited when
// there is no outstanding data to send. This means that certain bandwidth
// samples in the future would not be an accurate indication of the link
// capacity, and it is important to inform consumer about that. Whenever
// connection becomes app-limited, the sampler is notified via OnAppLimited()
// method.
//
// Meaning 2: a phase in the bandwidth sampler. As soon as the bandwidth
// sampler becomes notified about the connection being app-limited, it enters
// app-limited phase. In that phase, all *sent* packets are marked as
// app-limited. Note that the connection itself does not have to be
// app-limited during the app-limited phase, and in fact it will not be
// (otherwise how would it send packets?). The boolean flag below indicates
// whether the sampler is in that phase.
//
// Meaning 3: a flag on the sent packet and on the sample. If a sent packet is
// sent during the app-limited phase, the resulting sample related to the
// packet will be marked as app-limited.
//
// With the terminology issue out of the way, let us consider the question of
// what kind of situation it addresses.
//
// Consider a scenario where we first send packets 1 to 20 at a regular
// bandwidth, and then immediately run out of data. After a few seconds, we send
// packets 21 to 60, and only receive ack for 21 between sending packets 40 and
// 41. In this case, when we sample bandwidth for packets 21 to 40, the S_0/A_0
// we use to compute the slope is going to be packet 20, a few seconds apart
// from the current packet, hence the resulting estimate would be extremely low
// and not indicative of anything. Only at packet 41 the S_0/A_0 will become 21,
// meaning that the bandwidth sample would exclude the quiescence.
//
// Based on the analysis of that scenario, we implement the following rule: once
// OnAppLimited() is called, all sent packets will produce app-limited samples
// up until an ack for a packet that was sent after OnAppLimited() was called.
// Note that while the scenario above is not the only scenario when the
// connection is app-limited, the approach works in other cases too.
type BandwidthSampler struct {
	// The total number of congestion controlled bytes sent during the connection.
	totalBytesSent congestion.ByteCount
	// The total number of congestion controlled bytes which were acknowledged.
	totalBytesAcked congestion.ByteCount
	// The total number of congestion controlled bytes which were lost.
	totalBytesLost congestion.ByteCount
	// The value of |totalBytesSent| at the time the last acknowledged packet
	// was sent. Valid only when |lastAckedPacketSentTime| is valid.
	totalBytesSentAtLastAckedPacket congestion.ByteCount
	// The time at which the last acknowledged packet was sent. Set to
	// QuicTime::Zero() if no valid timestamp is available.
	lastAckedPacketSentTime time.Time
	// The time at which the most recent packet was acknowledged.
	lastAckedPacketAckTime time.Time
	// The most recently sent packet.
	lastSendPacket congestion.PacketNumber
	// Indicates whether the bandwidth sampler is currently in an app-limited
	// phase.
	isAppLimited bool
	// The packet that will be acknowledged after this one will cause the sampler
	// to exit the app-limited phase.
	endOfAppLimitedPhase congestion.PacketNumber
	// Record of the connection state at the point where each packet in flight was
	// sent, indexed by the packet number.
	connectionStats *ConnectionStates
}

func NewBandwidthSampler() *BandwidthSampler {
	return &BandwidthSampler{
		connectionStats: &ConnectionStates{
			stats: make(map[congestion.PacketNumber]*ConnectionStateOnSentPacket),
		},
	}
}

// OnPacketSent Inputs the sent packet information into the sampler. Assumes that all
// packets are sent in order. The information about the packet will not be
// released from the sampler until it the packet is either acknowledged or
// declared lost.
func (s *BandwidthSampler) OnPacketSent(sentTime time.Time, lastSentPacket congestion.PacketNumber, sentBytes, bytesInFlight congestion.ByteCount, hasRetransmittableData bool) {
	s.lastSendPacket = lastSentPacket

	if !hasRetransmittableData {
		return
	}

	s.totalBytesSent += sentBytes

	// If there are no packets in flight, the time at which the new transmission
	// opens can be treated as the A_0 point for the purpose of bandwidth
	// sampling. This underestimates bandwidth to some extent, and produces some
	// artificially low samples for most packets in flight, but it provides with
	// samples at important points where we would not have them otherwise, most
	// importantly at the beginning of the connection.
	if bytesInFlight == 0 {
		s.lastAckedPacketAckTime = sentTime
		s.totalBytesSentAtLastAckedPacket = s.totalBytesSent

		// In this situation ack compression is not a concern, set send rate to
		// effectively infinite.
		s.lastAckedPacketSentTime = sentTime
	}

	s.connectionStats.Insert(lastSentPacket, sentTime, sentBytes, s)
}

// OnPacketAcked Notifies the sampler that the |lastAckedPacket| is acknowledged. Returns a
// bandwidth sample. If no bandwidth sample is available,
// QuicBandwidth::Zero() is returned.
func (s *BandwidthSampler) OnPacketAcked(ackTime time.Time, lastAckedPacket congestion.PacketNumber) *BandwidthSample {
	sentPacketState := s.connectionStats.Get(lastAckedPacket)
	if sentPacketState == nil {
		return NewBandwidthSample()
	}

	sample := s.onPacketAckedInner(ackTime, lastAckedPacket, sentPacketState)
	s.connectionStats.Remove(lastAckedPacket)

	return sample
}

// onPacketAckedInner Handles the actual bandwidth calculations, whereas the outer method handles
// retrieving and removing |sentPacket|.
func (s *BandwidthSampler) onPacketAckedInner(ackTime time.Time, lastAckedPacket congestion.PacketNumber, sentPacket *ConnectionStateOnSentPacket) *BandwidthSample {
	s.totalBytesAcked += sentPacket.size

	s.totalBytesSentAtLastAckedPacket = sentPacket.sendTimeState.totalBytesSent
	s.lastAckedPacketSentTime = sentPacket.sendTime
	s.lastAckedPacketAckTime = ackTime

	// Exit app-limited phase once a packet that was sent while the connection is
	// not app-limited is acknowledged.
	if s.isAppLimited && lastAckedPacket > s.endOfAppLimitedPhase {
		s.isAppLimited = false
	}

	// There might have been no packets acknowledged at the moment when the
	// current packet was sent. In that case, there is no bandwidth sample to
	// make.
	if sentPacket.lastAckedPacketSentTime.IsZero() {
		return NewBandwidthSample()
	}

	// Infinite rate indicates that the sampler is supposed to discard the
	// current send rate sample and use only the ack rate.
	sendRate := InfiniteBandwidth
	if sentPacket.sendTime.After(sentPacket.lastAckedPacketSentTime) {
		sendRate = BandwidthFromDelta(sentPacket.sendTimeState.totalBytesSent-sentPacket.totalBytesSentAtLastAckedPacket, sentPacket.sendTime.Sub(sentPacket.lastAckedPacketSentTime))
	}

	// During the slope calculation, ensure that ack time of the current packet is
	// always larger than the time of the previous packet, otherwise division by
	// zero or integer underflow can occur.
	if !ackTime.After(sentPacket.lastAckedPacketAckTime) {
		// TODO(wub): Compare this code count before and after fixing clock jitter
		// issue.
		// if sentPacket.lastAckedPacketAckTime.Equal(sentPacket.sendTime) {
		// This is the 1st packet after quiescense.
		// QUIC_CODE_COUNT_N(quic_prev_ack_time_larger_than_current_ack_time, 1, 2);
		// } else {
		//   QUIC_CODE_COUNT_N(quic_prev_ack_time_larger_than_current_ack_time, 2, 2);
		// }

		return NewBandwidthSample()
	}

	ackRate := BandwidthFromDelta(s.totalBytesAcked-sentPacket.sendTimeState.totalBytesAcked,
		ackTime.Sub(sentPacket.lastAckedPacketAckTime))

	// Note: this sample does not account for delayed acknowledgement time.  This
	// means that the RTT measurements here can be artificially high, especially
	// on low bandwidth connections.
	sample := &BandwidthSample{
		bandwidth: minBandwidth(sendRate, ackRate),
		rtt:       ackTime.Sub(sentPacket.sendTime),
	}

	SentPacketToSendTimeState(sentPacket, &sample.stateAtSend)
	return sample
}

// OnPacketLost Informs the sampler that a packet is considered lost and it should no
// longer keep track of it.
func (s *BandwidthSampler) OnPacketLost(packetNumber congestion.PacketNumber) SendTimeState {
	ok, sentPacket := s.connectionStats.Remove(packetNumber)
	sendTimeState := SendTimeState{
		isValid: ok,
	}
	if sentPacket != nil {
		s.totalBytesLost += sentPacket.size
		SentPacketToSendTimeState(sentPacket, &sendTimeState)
	}

	return sendTimeState
}

// OnAppLimited Informs the sampler that the connection is currently app-limited, causing
// the sampler to enter the app-limited phase.  The phase will expire by
// itself.
func (s *BandwidthSampler) OnAppLimited() {
	s.isAppLimited = true
	s.endOfAppLimitedPhase = s.lastSendPacket
}

// SentPacketToSendTimeState Copy a subset of the (private) ConnectionStateOnSentPacket to the (public)
// SendTimeState. Always set send_time_state->is_valid to true.
func SentPacketToSendTimeState(sentPacket *ConnectionStateOnSentPacket, sendTimeState *SendTimeState) {
	sendTimeState.isAppLimited = sentPacket.sendTimeState.isAppLimited
	sendTimeState.totalBytesSent = sentPacket.sendTimeState.totalBytesSent
	sendTimeState.totalBytesAcked = sentPacket.sendTimeState.totalBytesAcked
	sendTimeState.totalBytesLost = sentPacket.sendTimeState.totalBytesLost
	sendTimeState.isValid = true
}

// ConnectionStates Record of the connection state at the point where each packet in flight was
// sent, indexed by the packet number.
// FIXME: using LinkedList replace map to fast remove all the packets lower than the specified packet number.
type ConnectionStates struct {
	stats map[congestion.PacketNumber]*ConnectionStateOnSentPacket
}

func (s *ConnectionStates) Insert(packetNumber congestion.PacketNumber, sentTime time.Time, bytes congestion.ByteCount, sampler *BandwidthSampler) bool {
	if _, ok := s.stats[packetNumber]; ok {
		return false
	}

	s.stats[packetNumber] = NewConnectionStateOnSentPacket(packetNumber, sentTime, bytes, sampler)
	return true
}

func (s *ConnectionStates) Get(packetNumber congestion.PacketNumber) *ConnectionStateOnSentPacket {
	return s.stats[packetNumber]
}

func (s *ConnectionStates) Remove(packetNumber congestion.PacketNumber) (bool, *ConnectionStateOnSentPacket) {
	state, ok := s.stats[packetNumber]
	if ok {
		delete(s.stats, packetNumber)
	}
	return ok, state
}

func NewConnectionStateOnSentPacket(packetNumber congestion.PacketNumber, sentTime time.Time, bytes congestion.ByteCount, sampler *BandwidthSampler) *ConnectionStateOnSentPacket {
	return &ConnectionStateOnSentPacket{
		packetNumber:                    packetNumber,
		sendTime:                        sentTime,
		size:                            bytes,
		lastAckedPacketSentTime:         sampler.lastAckedPacketSentTime,
		lastAckedPacketAckTime:          sampler.lastAckedPacketAckTime,
		totalBytesSentAtLastAckedPacket: sampler.totalBytesSentAtLastAckedPacket,
		sendTimeState: SendTimeState{
			isValid:         true,
			isAppLimited:    sampler.isAppLimited,
			totalBytesSent:  sampler.totalBytesSent,
			totalBytesAcked: sampler.totalBytesAcked,
			totalBytesLost:  sampler.totalBytesLost,
		},
	}
}
