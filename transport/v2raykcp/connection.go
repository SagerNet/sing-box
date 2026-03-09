package v2raykcp

import (
	"bytes"
	"io"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing/common/buf"
)

// PacketWriter writes low-level UDP packets with obfuscating header and AEAD.
// It mirrors v2ray-core's kcp.PacketWriter.
type PacketWriter interface {
	Overhead() int
	io.Writer
}

// State of the connection
type State int32

const (
	StateActive          State = 0
	StateReadyToClose    State = 1
	StatePeerClosed      State = 2
	StateTerminating     State = 3
	StatePeerTerminating State = 4
	StateTerminated      State = 5
)

// Is returns true if current State is one of the candidates.
func (s State) Is(states ...State) bool {
	for _, state := range states {
		if s == state {
			return true
		}
	}
	return false
}

func nowMillisec() int64 {
	now := time.Now()
	return now.Unix()*1000 + int64(now.Nanosecond()/1000000)
}

// RoundTripInfo stores round trip time information
type RoundTripInfo struct {
	mu               sync.RWMutex
	variation        uint32
	srtt             uint32
	rto              uint32
	minRtt           uint32
	updatedTimestamp uint32
}

func (info *RoundTripInfo) UpdatePeerRTO(rto uint32, current uint32) {
	info.mu.Lock()
	defer info.mu.Unlock()

	if current-info.updatedTimestamp < 3000 {
		return
	}
	info.updatedTimestamp = current
	info.rto = rto
}

func (info *RoundTripInfo) Update(rtt uint32, current uint32) {
	if rtt > 0x7FFFFFFF {
		return
	}

	info.mu.Lock()
	defer info.mu.Unlock()

	if info.srtt == 0 {
		info.srtt = rtt
		info.variation = rtt / 2
	} else {
		delta := rtt - info.srtt
		if info.srtt > rtt {
			delta = info.srtt - rtt
		}
		info.variation = (3*info.variation + delta) / 4
		info.srtt = (7*info.srtt + rtt) / 8
		if info.srtt < info.minRtt {
			info.srtt = info.minRtt
		}
	}

	var rto uint32
	if info.minRtt < 4*info.variation {
		rto = info.srtt + 4*info.variation
	} else {
		rto = info.srtt + info.variation
	}

	if rto > 10000 {
		rto = 10000
	}
	info.rto = rto * 5 / 4
	info.updatedTimestamp = current
}

func (info *RoundTripInfo) Timeout() uint32 {
	info.mu.RLock()
	defer info.mu.RUnlock()

	if info.rto == 0 {
		return 100
	}
	return info.rto
}

func (info *RoundTripInfo) SmoothedTime() uint32 {
	info.mu.RLock()
	defer info.mu.RUnlock()

	return info.srtt
}

// ConnMetadata stores connection metadata
type ConnMetadata struct {
	LocalAddr    net.Addr
	RemoteAddr   net.Addr
	Conversation uint16
}

// Connection represents a KCP connection
type Connection struct {
	meta             ConnMetadata
	closer           io.Closer
	rd               time.Time
	wd               time.Time
	since            int64
	dataInput        chan struct{}
	dataOutput       chan struct{}
	Config           *Config
	state            int32
	stateBeginTime   uint32
	lastIncomingTime uint32
	lastPingTime     uint32
	mss              uint32
	roundTrip        *RoundTripInfo
	receivingWorker  *ReceivingWorker
	sendingWorker    *SendingWorker
	output           SegmentWriter
	dataUpdater      *Updater
	pingUpdater      *Updater
}

func NewConnection(meta ConnMetadata, writer PacketWriter, closer io.Closer, config *Config) *Connection {
	conn := &Connection{
		meta:       meta,
		closer:     closer,
		since:      nowMillisec(),
		dataInput:  make(chan struct{}, 1),
		dataOutput: make(chan struct{}, 1),
		Config:     config,
		output:     NewSegmentWriter(writer),
		mss:        config.GetMTUValue() - uint32(writer.Overhead()) - uint32(DataSegmentOverhead),
		roundTrip: &RoundTripInfo{
			rto:    100,
			minRtt: config.GetTTIValue(),
		},
	}

	conn.receivingWorker = NewReceivingWorker(conn)
	conn.sendingWorker = NewSendingWorker(conn)

	isTerminating := func() bool {
		return conn.State().Is(StateTerminating, StateTerminated)
	}
	isTerminated := func() bool {
		return conn.State() == StateTerminated
	}
	
	conn.dataUpdater = NewUpdater(
		config.GetTTIValue(),
		func() bool {
			return !isTerminating() && (conn.sendingWorker.UpdateNecessary() || conn.receivingWorker.UpdateNecessary())
		},
		isTerminating,
		conn.updateTask)
	conn.pingUpdater = NewUpdater(
		5000,
		func() bool { return !isTerminated() },
		isTerminated,
		conn.updateTask)
	conn.pingUpdater.WakeUp()

	return conn
}

func (c *Connection) Elapsed() uint32 {
	return uint32(nowMillisec() - c.since)
}

func (c *Connection) State() State {
	return State(atomic.LoadInt32(&c.state))
}

func (c *Connection) SetState(state State) {
	current := c.Elapsed()
	atomic.StoreInt32(&c.state, int32(state))
	atomic.StoreUint32(&c.stateBeginTime, current)

	switch state {
	case StateReadyToClose:
		c.receivingWorker.CloseRead()
	case StatePeerClosed:
		c.sendingWorker.CloseWrite()
	case StateTerminating:
		c.receivingWorker.CloseRead()
		c.sendingWorker.CloseWrite()
		c.pingUpdater.SetInterval(time.Second)
	case StatePeerTerminating:
		c.sendingWorker.CloseWrite()
		c.pingUpdater.SetInterval(time.Second)
	case StateTerminated:
		c.receivingWorker.CloseRead()
		c.sendingWorker.CloseWrite()
		c.pingUpdater.SetInterval(time.Second)
		c.dataUpdater.WakeUp()
		c.pingUpdater.WakeUp()
		go c.Terminate()
	}
}

func (c *Connection) Terminate() {
	if c == nil {
		return
	}
	time.Sleep(8 * time.Second)

	if c.closer != nil {
		c.closer.Close()
	}
	if c.sendingWorker != nil {
		c.sendingWorker.Release()
	}
	if c.receivingWorker != nil {
		c.receivingWorker.Release()
	}
}

func (c *Connection) HandleOption(opt SegmentOption) {
	if (opt & SegmentOptionClose) == SegmentOptionClose {
		c.OnPeerClosed()
	}
}

func (c *Connection) OnPeerClosed() {
	switch c.State() {
	case StateReadyToClose:
		c.SetState(StateTerminating)
	case StateActive:
		c.SetState(StatePeerClosed)
	}
}

func (c *Connection) Input(segments []Segment) {
	current := c.Elapsed()
	atomic.StoreUint32(&c.lastIncomingTime, current)

	for _, s := range segments {
		if s.Conversation() != c.meta.Conversation {
			break
		}

		switch seg := s.(type) {
		case *DataSegment:
			c.HandleOption(seg.Option)
			c.receivingWorker.ProcessSegment(seg)
			if c.receivingWorker.IsDataAvailable() {
				select {
				case c.dataInput <- struct{}{}:
				default:
				}
			}
			c.dataUpdater.WakeUp()
		case *AckSegment:
			c.HandleOption(seg.Option)
			c.sendingWorker.ProcessSegment(current, seg, c.roundTrip.Timeout())
			select {
			case c.dataOutput <- struct{}{}:
			default:
			}
			c.dataUpdater.WakeUp()
		case *CmdOnlySegment:
			c.HandleOption(seg.Option)
			if seg.Command() == CommandTerminate {
				switch c.State() {
				case StateActive, StatePeerClosed:
					c.SetState(StatePeerTerminating)
				case StateReadyToClose:
					c.SetState(StateTerminating)
				case StateTerminating:
					c.SetState(StateTerminated)
				}
			}
			if seg.Option == SegmentOptionClose || seg.Command() == CommandTerminate {
				select {
				case c.dataInput <- struct{}{}:
				default:
				}
				select {
				case c.dataOutput <- struct{}{}:
				default:
				}
			}
			c.sendingWorker.ProcessReceivingNext(seg.ReceivingNext)
			c.receivingWorker.ProcessSendingNext(seg.SendingNext)
			c.roundTrip.UpdatePeerRTO(seg.PeerRTO, current)
			seg.Release()
		default:
			s.Release()
		}
	}
}

func (c *Connection) waitForDataInput() error {
	for i := 0; i < 16; i++ {
		select {
		case <-c.dataInput:
			return nil
		default:
			runtime.Gosched()
		}
	}

	duration := time.Second * 16
	if !c.rd.IsZero() {
		duration = time.Until(c.rd)
		if duration < 0 {
			return ErrIOTimeout
		}
	}

	select {
	case <-c.dataInput:
		return nil
	case <-time.After(duration):
		if !c.rd.IsZero() && c.rd.Before(time.Now()) {
			return ErrIOTimeout
		}
		return nil
	}
}

func (c *Connection) Read(b []byte) (int, error) {
	if c == nil {
		return 0, io.EOF
	}

	for {
		if c.State().Is(StateReadyToClose, StateTerminating, StateTerminated) {
			return 0, io.EOF
		}
		
		nBytes := c.receivingWorker.Read(b)
		if nBytes > 0 {
			c.dataUpdater.WakeUp()
			return nBytes, nil
		}

		if c.State() == StatePeerTerminating {
			return 0, io.EOF
		}

		if err := c.waitForDataInput(); err != nil {
			return 0, err
		}
	}
}

func (c *Connection) waitForDataOutput() error {
	for i := 0; i < 16; i++ {
		select {
		case <-c.dataOutput:
			return nil
		default:
			runtime.Gosched()
		}
	}

	duration := time.Second * 16
	if !c.wd.IsZero() {
		duration = time.Until(c.wd)
		if duration < 0 {
			return ErrIOTimeout
		}
	}

	select {
	case <-c.dataOutput:
		return nil
	case <-time.After(duration):
		if !c.wd.IsZero() && c.wd.Before(time.Now()) {
			return ErrIOTimeout
		}
		return nil
	}
}

func (c *Connection) Write(b []byte) (int, error) {
	if c.State() != StateActive {
		return 0, io.ErrClosedPipe
	}

	totalWritten := 0
	reader := bytes.NewReader(b)

	for reader.Len() > 0 {
		buffer := buf.New()
		n, _ := buffer.ReadFrom(io.LimitReader(reader, int64(c.mss)))
		if n == 0 {
			buffer.Release()
			break
		}

		for !c.sendingWorker.Push(buffer) {
			if c.State() != StateActive {
				buffer.Release()
				return totalWritten, io.ErrClosedPipe
			}

			c.dataUpdater.WakeUp()

			if err := c.waitForDataOutput(); err != nil {
				buffer.Release()
				return totalWritten, err
			}
		}

		totalWritten += int(n)
	}

	c.dataUpdater.WakeUp()
	return totalWritten, nil
}

func (c *Connection) updateTask() {
	current := c.Elapsed()

	if c.State() == StateTerminated {
		return
	}
	if c.State() == StateActive && current-atomic.LoadUint32(&c.lastIncomingTime) >= 30000 {
		_ = c.Close()
	}
	if c.State() == StateReadyToClose && c.sendingWorker.IsEmpty() {
		c.SetState(StateTerminating)
	}
	if c.State() == StateTerminating {
		if current-atomic.LoadUint32(&c.stateBeginTime) > 8000 {
			c.SetState(StateTerminated)
		} else {
			c.Ping(current, CommandTerminate)
		}
		return
	}
	if c.State() == StatePeerTerminating && current-atomic.LoadUint32(&c.stateBeginTime) > 4000 {
		c.SetState(StateTerminating)
	}
	if c.State() == StateReadyToClose && current-atomic.LoadUint32(&c.stateBeginTime) > 15000 {
		c.SetState(StateTerminating)
	}

	c.receivingWorker.Flush(current)
	c.sendingWorker.Flush(current)

	if current-atomic.LoadUint32(&c.lastPingTime) >= 3000 {
		c.Ping(current, CommandPing)
	}

	select {
	case c.dataOutput <- struct{}{}:
	default:
	}
}

func (c *Connection) Close() error {
	if c == nil {
		return ErrClosedConnection
	}

	select {
	case c.dataInput <- struct{}{}:
	default:
	}
	select {
	case c.dataOutput <- struct{}{}:
	default:
	}

	switch c.State() {
	case StateReadyToClose, StateTerminating, StateTerminated:
		return ErrClosedConnection
	case StateActive:
		c.SetState(StateReadyToClose)
	case StatePeerClosed:
		c.SetState(StateTerminating)
	case StatePeerTerminating:
		c.SetState(StateTerminated)
	}

	return nil
}

func (c *Connection) LocalAddr() net.Addr {
	if c == nil {
		return nil
	}
	return c.meta.LocalAddr
}

func (c *Connection) RemoteAddr() net.Addr {
	if c == nil {
		return nil
	}
	return c.meta.RemoteAddr
}

func (c *Connection) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	if err := c.SetWriteDeadline(t); err != nil {
		return err
	}
	return nil
}

func (c *Connection) SetReadDeadline(t time.Time) error {
	if c == nil {
		return ErrClosedConnection
	}
	c.rd = t
	return nil
}

func (c *Connection) SetWriteDeadline(t time.Time) error {
	if c == nil {
		return ErrClosedConnection
	}
	c.wd = t
	return nil
}

func (c *Connection) Ping(current uint32, cmd Command) {
	seg := NewCmdOnlySegment()
	seg.Conv = c.meta.Conversation
	seg.Cmd = cmd
	seg.SendingNext = c.sendingWorker.FirstUnacknowledged()
	seg.ReceivingNext = c.receivingWorker.NextNumber()
	seg.PeerRTO = c.roundTrip.Timeout()
	if c.State() == StateReadyToClose {
		seg.Option = SegmentOptionClose
	}
	c.output.Write(seg)
	atomic.StoreUint32(&c.lastPingTime, current)
	seg.Release()
}
