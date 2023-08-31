package hysteria2

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/quicvarint"
	"github.com/sagernet/sing-box/transport/hysteria2/internal/protocol"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/cache"
	M "github.com/sagernet/sing/common/metadata"
)

var udpMessagePool = sync.Pool{
	New: func() interface{} {
		return new(udpMessage)
	},
}

func releaseMessages(messages []*udpMessage) {
	for _, message := range messages {
		if message != nil {
			*message = udpMessage{}
			udpMessagePool.Put(message)
		}
	}
}

type udpMessage struct {
	sessionID     uint32
	packetID      uint16
	fragmentID    uint8
	fragmentTotal uint8
	destination   string
	data          *buf.Buffer
}

func (m *udpMessage) release() {
	*m = udpMessage{}
	udpMessagePool.Put(m)
}

func (m *udpMessage) releaseMessage() {
	m.data.Release()
	m.release()
}

func (m *udpMessage) pack() *buf.Buffer {
	buffer := buf.NewSize(m.headerSize() + m.data.Len())
	common.Must(
		binary.Write(buffer, binary.BigEndian, m.sessionID),
		binary.Write(buffer, binary.BigEndian, m.packetID),
		binary.Write(buffer, binary.BigEndian, m.fragmentID),
		binary.Write(buffer, binary.BigEndian, m.fragmentTotal),
		protocol.WriteVString(buffer, m.destination),
		common.Error(buffer.Write(m.data.Bytes())),
	)
	return buffer
}

func (m *udpMessage) headerSize() int {
	return 8 + int(quicvarint.Len(uint64(len(m.destination)))) + len(m.destination)
}

func fragUDPMessage(message *udpMessage, maxPacketSize int) []*udpMessage {
	if message.data.Len() <= maxPacketSize {
		return []*udpMessage{message}
	}
	var fragments []*udpMessage
	originPacket := message.data.Bytes()
	udpMTU := maxPacketSize - message.headerSize()
	for remaining := len(originPacket); remaining > 0; remaining -= udpMTU {
		fragment := udpMessagePool.Get().(*udpMessage)
		*fragment = *message
		if remaining > udpMTU {
			fragment.data = buf.As(originPacket[:udpMTU])
			originPacket = originPacket[udpMTU:]
		} else {
			fragment.data = buf.As(originPacket)
			originPacket = nil
		}
		fragments = append(fragments, fragment)
	}
	fragmentTotal := uint16(len(fragments))
	for index, fragment := range fragments {
		fragment.fragmentID = uint8(index)
		fragment.fragmentTotal = uint8(fragmentTotal)
		/*if index > 0 {
			fragment.destination = ""
			// not work in hysteria
		}*/
	}
	return fragments
}

type udpPacketConn struct {
	ctx        context.Context
	cancel     common.ContextCancelCauseFunc
	sessionID  uint32
	quicConn   quic.Connection
	data       chan *udpMessage
	udpMTU     int
	udpMTUTime time.Time
	packetId   atomic.Uint32
	closeOnce  sync.Once
	defragger  *udpDefragger
	onDestroy  func()
}

func newUDPPacketConn(ctx context.Context, quicConn quic.Connection, onDestroy func()) *udpPacketConn {
	ctx, cancel := common.ContextWithCancelCause(ctx)
	return &udpPacketConn{
		ctx:       ctx,
		cancel:    cancel,
		quicConn:  quicConn,
		data:      make(chan *udpMessage, 64),
		defragger: newUDPDefragger(),
		onDestroy: onDestroy,
	}
}

func (c *udpPacketConn) ReadPacketThreadSafe() (buffer *buf.Buffer, destination M.Socksaddr, err error) {
	select {
	case p := <-c.data:
		buffer = p.data
		destination = M.ParseSocksaddr(p.destination)
		p.release()
		return
	case <-c.ctx.Done():
		return nil, M.Socksaddr{}, io.ErrClosedPipe
	}
}

func (c *udpPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	select {
	case p := <-c.data:
		_, err = buffer.ReadOnceFrom(p.data)
		destination = M.ParseSocksaddr(p.destination)
		p.releaseMessage()
		return
	case <-c.ctx.Done():
		return M.Socksaddr{}, io.ErrClosedPipe
	}
}

func (c *udpPacketConn) WaitReadPacket(newBuffer func() *buf.Buffer) (destination M.Socksaddr, err error) {
	select {
	case p := <-c.data:
		_, err = newBuffer().ReadOnceFrom(p.data)
		destination = M.ParseSocksaddr(p.destination)
		p.releaseMessage()
		return
	case <-c.ctx.Done():
		return M.Socksaddr{}, io.ErrClosedPipe
	}
}

func (c *udpPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	select {
	case pkt := <-c.data:
		n = copy(p, pkt.data.Bytes())
		destination := M.ParseSocksaddr(pkt.destination)
		if destination.IsFqdn() {
			addr = destination
		} else {
			addr = destination.UDPAddr()
		}
		pkt.releaseMessage()
		return n, addr, nil
	case <-c.ctx.Done():
		return 0, nil, io.ErrClosedPipe
	}
}

func (c *udpPacketConn) needFragment() bool {
	nowTime := time.Now()
	if c.udpMTU > 0 && nowTime.Sub(c.udpMTUTime) < 5*time.Second {
		c.udpMTUTime = nowTime
		return true
	}
	return false
}

func (c *udpPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	select {
	case <-c.ctx.Done():
		return net.ErrClosed
	default:
	}
	if buffer.Len() > 0xffff {
		return quic.ErrMessageTooLarge(0xffff)
	}
	packetId := c.packetId.Add(1)
	if packetId > math.MaxUint16 {
		c.packetId.Store(0)
		packetId = 0
	}
	message := udpMessagePool.Get().(*udpMessage)
	*message = udpMessage{
		sessionID:     c.sessionID,
		packetID:      uint16(packetId),
		fragmentTotal: 1,
		destination:   destination.String(),
		data:          buffer,
	}
	defer message.releaseMessage()
	var err error
	if c.needFragment() && buffer.Len() > c.udpMTU {
		err = c.writePackets(fragUDPMessage(message, c.udpMTU))
	} else {
		err = c.writePacket(message)
	}
	if err == nil {
		return nil
	}
	var tooLargeErr quic.ErrMessageTooLarge
	if !errors.As(err, &tooLargeErr) {
		return err
	}
	c.udpMTU = int(tooLargeErr)
	c.udpMTUTime = time.Now()
	return c.writePackets(fragUDPMessage(message, c.udpMTU))
}

func (c *udpPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	select {
	case <-c.ctx.Done():
		return 0, net.ErrClosed
	default:
	}
	if len(p) > 0xffff {
		return 0, quic.ErrMessageTooLarge(0xffff)
	}
	packetId := c.packetId.Add(1)
	if packetId > math.MaxUint16 {
		c.packetId.Store(0)
		packetId = 0
	}
	message := udpMessagePool.Get().(*udpMessage)
	*message = udpMessage{
		sessionID:     c.sessionID,
		packetID:      uint16(packetId),
		fragmentTotal: 1,
		destination:   addr.String(),
		data:          buf.As(p),
	}
	if c.needFragment() && len(p) > c.udpMTU {
		err = c.writePackets(fragUDPMessage(message, c.udpMTU))
		if err == nil {
			return len(p), nil
		}
	} else {
		err = c.writePacket(message)
	}
	if err == nil {
		return len(p), nil
	}
	var tooLargeErr quic.ErrMessageTooLarge
	if !errors.As(err, &tooLargeErr) {
		return
	}
	c.udpMTU = int(tooLargeErr)
	c.udpMTUTime = time.Now()
	err = c.writePackets(fragUDPMessage(message, c.udpMTU))
	if err == nil {
		return len(p), nil
	}
	return
}

func (c *udpPacketConn) inputPacket(message *udpMessage) {
	if message.fragmentTotal <= 1 {
		select {
		case c.data <- message:
		default:
		}
	} else {
		newMessage := c.defragger.feed(message)
		if newMessage != nil {
			select {
			case c.data <- newMessage:
			default:
			}
		}
	}
}

func (c *udpPacketConn) writePackets(messages []*udpMessage) error {
	defer releaseMessages(messages)
	for _, message := range messages {
		err := c.writePacket(message)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *udpPacketConn) writePacket(message *udpMessage) error {
	buffer := message.pack()
	defer buffer.Release()
	return c.quicConn.SendMessage(buffer.Bytes())
}

func (c *udpPacketConn) Close() error {
	c.closeOnce.Do(func() {
		c.closeWithError(os.ErrClosed)
		c.onDestroy()
	})
	return nil
}

func (c *udpPacketConn) closeWithError(err error) {
	c.cancel(err)
}

func (c *udpPacketConn) LocalAddr() net.Addr {
	return c.quicConn.LocalAddr()
}

func (c *udpPacketConn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *udpPacketConn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *udpPacketConn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}

type udpDefragger struct {
	packetMap *cache.LruCache[uint16, *packetItem]
}

func newUDPDefragger() *udpDefragger {
	return &udpDefragger{
		packetMap: cache.New(
			cache.WithAge[uint16, *packetItem](10),
			cache.WithUpdateAgeOnGet[uint16, *packetItem](),
			cache.WithEvict[uint16, *packetItem](func(key uint16, value *packetItem) {
				releaseMessages(value.messages)
			}),
		),
	}
}

type packetItem struct {
	access   sync.Mutex
	messages []*udpMessage
	count    uint8
}

func (d *udpDefragger) feed(m *udpMessage) *udpMessage {
	if m.fragmentTotal <= 1 {
		return m
	}
	if m.fragmentID >= m.fragmentTotal {
		return nil
	}
	item, _ := d.packetMap.LoadOrStore(m.packetID, newPacketItem)
	item.access.Lock()
	defer item.access.Unlock()
	if int(m.fragmentTotal) != len(item.messages) {
		releaseMessages(item.messages)
		item.messages = make([]*udpMessage, m.fragmentTotal)
		item.count = 1
		item.messages[m.fragmentID] = m
		return nil
	}
	if item.messages[m.fragmentID] != nil {
		return nil
	}
	item.messages[m.fragmentID] = m
	item.count++
	if int(item.count) != len(item.messages) {
		return nil
	}
	newMessage := udpMessagePool.Get().(*udpMessage)
	*newMessage = *item.messages[0]
	var finalLength int
	for _, message := range item.messages {
		finalLength += message.data.Len()
	}
	if finalLength > 0 {
		newMessage.data = buf.NewSize(finalLength)
		for _, message := range item.messages {
			newMessage.data.Write(message.data.Bytes())
			message.releaseMessage()
		}
		item.messages = nil
		return newMessage
	}
	return nil
}

func newPacketItem() *packetItem {
	return new(packetItem)
}

func decodeUDPMessage(message *udpMessage, data []byte) error {
	reader := bytes.NewReader(data)
	err := binary.Read(reader, binary.BigEndian, &message.sessionID)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &message.packetID)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &message.fragmentID)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &message.fragmentTotal)
	if err != nil {
		return err
	}
	message.destination, err = protocol.ReadVString(reader)
	if err != nil {
		return err
	}
	message.data = buf.As(data[len(data)-reader.Len():])
	return nil
}
