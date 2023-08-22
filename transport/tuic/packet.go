package tuic

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
	sessionID     uint16
	packetID      uint16
	fragmentTotal uint8
	fragmentID    uint8
	destination   M.Socksaddr
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
		buffer.WriteByte(Version),
		buffer.WriteByte(CommandPacket),
		binary.Write(buffer, binary.BigEndian, m.sessionID),
		binary.Write(buffer, binary.BigEndian, m.packetID),
		binary.Write(buffer, binary.BigEndian, m.fragmentTotal),
		binary.Write(buffer, binary.BigEndian, m.fragmentID),
		binary.Write(buffer, binary.BigEndian, uint16(m.data.Len())),
		addressSerializer.WriteAddrPort(buffer, m.destination),
		common.Error(buffer.Write(m.data.Bytes())),
	)
	return buffer
}

func (m *udpMessage) headerSize() int {
	return 10 + addressSerializer.AddrPortLen(m.destination)
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
		if index > 0 {
			fragment.destination = M.Socksaddr{}
		}
	}
	return fragments
}

type udpPacketConn struct {
	ctx        context.Context
	cancel     common.ContextCancelCauseFunc
	sessionID  uint16
	quicConn   quic.Connection
	data       chan *udpMessage
	udpStream  bool
	udpMTU     int
	udpMTUTime time.Time
	packetId   atomic.Uint32
	closeOnce  sync.Once
	isServer   bool
	defragger  *udpDefragger
	onDestroy  func()
}

func newUDPPacketConn(ctx context.Context, quicConn quic.Connection, udpStream bool, isServer bool, onDestroy func()) *udpPacketConn {
	ctx, cancel := common.ContextWithCancelCause(ctx)
	return &udpPacketConn{
		ctx:       ctx,
		cancel:    cancel,
		quicConn:  quicConn,
		data:      make(chan *udpMessage, 64),
		udpStream: udpStream,
		isServer:  isServer,
		defragger: newUDPDefragger(),
		onDestroy: onDestroy,
	}
}

func (c *udpPacketConn) ReadPacketThreadSafe() (buffer *buf.Buffer, destination M.Socksaddr, err error) {
	select {
	case p := <-c.data:
		buffer = p.data
		destination = p.destination
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
		destination = p.destination
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
		destination = p.destination
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
		if pkt.destination.IsFqdn() {
			addr = pkt.destination
		} else {
			addr = pkt.destination.UDPAddr()
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
		destination:   destination,
		data:          buffer,
	}
	defer message.releaseMessage()
	var err error
	if !c.udpStream && c.needFragment() && buffer.Len() > c.udpMTU {
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
		destination:   M.SocksaddrFromNet(addr),
		data:          buf.As(p),
	}
	if !c.udpStream && c.needFragment() && len(p) > c.udpMTU {
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
	if !c.udpStream {
		buffer := message.pack()
		err := c.quicConn.SendMessage(buffer.Bytes())
		buffer.Release()
		if err != nil {
			return err
		}
	} else {
		stream, err := c.quicConn.OpenUniStream()
		if err != nil {
			return err
		}
		buffer := message.pack()
		_, err = stream.Write(buffer.Bytes())
		buffer.Release()
		stream.Close()
		if err != nil {
			return err
		}
	}
	return nil
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
	if !c.isServer {
		buffer := buf.NewSize(4)
		defer buffer.Release()
		buffer.WriteByte(Version)
		buffer.WriteByte(CommandDissociate)
		binary.Write(buffer, binary.BigEndian, c.sessionID)
		sendStream, openErr := c.quicConn.OpenUniStream()
		if openErr != nil {
			return
		}
		defer sendStream.Close()
		sendStream.Write(buffer.Bytes())
	}
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
	var dataLength uint16
	for _, message := range item.messages {
		dataLength += uint16(message.data.Len())
	}
	if dataLength > 0 {
		newMessage.data = buf.NewSize(int(dataLength))
		for _, message := range item.messages {
			common.Must1(newMessage.data.Write(message.data.Bytes()))
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

func readUDPMessage(message *udpMessage, reader io.Reader) error {
	err := binary.Read(reader, binary.BigEndian, &message.sessionID)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &message.packetID)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &message.fragmentTotal)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &message.fragmentID)
	if err != nil {
		return err
	}
	var dataLength uint16
	err = binary.Read(reader, binary.BigEndian, &dataLength)
	if err != nil {
		return err
	}
	message.destination, err = addressSerializer.ReadAddrPort(reader)
	if err != nil {
		return err
	}
	message.data = buf.NewSize(int(dataLength))
	_, err = message.data.ReadFullFrom(reader, message.data.FreeLen())
	if err != nil {
		return err
	}
	return nil
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
	err = binary.Read(reader, binary.BigEndian, &message.fragmentTotal)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &message.fragmentID)
	if err != nil {
		return err
	}
	var dataLength uint16
	err = binary.Read(reader, binary.BigEndian, &dataLength)
	if err != nil {
		return err
	}
	message.destination, err = addressSerializer.ReadAddrPort(reader)
	if err != nil {
		return err
	}
	if reader.Len() != int(dataLength) {
		return io.ErrUnexpectedEOF
	}
	message.data = buf.As(data[len(data)-reader.Len():])
	return nil
}
