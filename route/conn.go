package route

import (
	"context"
	"io"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/sniff"
	"github.com/sagernet/sing-box/common/tlsfragment"
	"github.com/sagernet/sing-box/common/tlsspoof"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/canceler"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
)

var _ adapter.ConnectionManager = (*ConnectionManager)(nil)

type ConnectionManager struct {
	logger      logger.ContextLogger
	access      sync.Mutex
	connections list.List[io.Closer]
}

func NewConnectionManager(logger logger.ContextLogger) *ConnectionManager {
	return &ConnectionManager{
		logger: logger,
	}
}

func (m *ConnectionManager) Start(stage adapter.StartStage) error {
	return nil
}

func (m *ConnectionManager) Count() int {
	m.access.Lock()
	defer m.access.Unlock()
	return m.connections.Len()
}

func (m *ConnectionManager) CloseAll() {
	m.access.Lock()
	var closers []io.Closer
	for element := m.connections.Front(); element != nil; {
		nextElement := element.Next()
		closers = append(closers, element.Value)
		m.connections.Remove(element)
		element = nextElement
	}
	m.access.Unlock()
	for _, closer := range closers {
		common.Close(closer)
	}
}

func (m *ConnectionManager) Close() error {
	m.CloseAll()
	return nil
}

func (m *ConnectionManager) TrackConn(conn net.Conn) net.Conn {
	tracked := &trackedConn{
		Conn:        conn,
		socketOwner: socketOwner{original: conn},
		manager:     m,
	}
	m.access.Lock()
	tracked.element = m.connections.PushBack(tracked)
	m.access.Unlock()
	return tracked
}

func (m *ConnectionManager) TrackPacketConn(conn net.PacketConn) net.PacketConn {
	tracked := &trackedPacketConn{
		NetPacketConn: bufio.NewPacketConn(conn),
		socketOwner:   socketOwner{original: conn},
		manager:       m,
	}
	m.access.Lock()
	tracked.element = m.connections.PushBack(tracked)
	m.access.Unlock()
	return tracked
}

func (m *ConnectionManager) NewConnection(ctx context.Context, this N.Dialer, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = adapter.WithContext(ctx, &metadata)
	var (
		remoteConn net.Conn
		err        error
	)
	if len(metadata.DestinationAddresses) > 0 || metadata.Destination.IsIP() {
		remoteConn, err = dialer.DialSerialNetwork(ctx, this, N.NetworkTCP, metadata.Destination, metadata.DestinationAddresses, metadata.NetworkStrategy, metadata.NetworkType, metadata.FallbackNetworkType, metadata.FallbackDelay)
	} else {
		remoteConn, err = this.DialContext(ctx, N.NetworkTCP, metadata.Destination)
	}
	if err != nil {
		var remoteString string
		if len(metadata.DestinationAddresses) > 0 {
			remoteString = "[" + strings.Join(common.Map(metadata.DestinationAddresses, netip.Addr.String), ",") + "]"
		} else {
			remoteString = metadata.Destination.String()
		}
		var dialerString string
		if outbound, isOutbound := this.(adapter.Outbound); isOutbound {
			dialerString = " using outbound/" + outbound.Type() + "[" + outbound.Tag() + "]"
		}
		err = E.Cause(err, "open connection to ", remoteString, dialerString)
		N.CloseOnHandshakeFailure(conn, onClose, err)
		m.logger.ErrorContext(ctx, err)
		return
	}
	err = N.ReportConnHandshakeSuccess(conn, remoteConn)
	if err != nil {
		err = E.Cause(err, "report handshake success")
		remoteConn.Close()
		N.CloseOnHandshakeFailure(conn, onClose, err)
		m.logger.ErrorContext(ctx, err)
		return
	}
	if !metadata.TLSFragment && !metadata.TLSRecordFragment && metadata.TLSSpoof == "" {
		var spliced bool
		spliced, err = m.spliceConnection(ctx, conn, remoteConn, onClose)
		if err != nil {
			N.CloseOnHandshakeFailure(conn, onClose, err)
			remoteConn.Close()
			m.logger.ErrorContext(ctx, err)
			return
		}
		if spliced {
			return
		}
	}
	if metadata.TLSFragment || metadata.TLSRecordFragment {
		remoteConn = tf.NewConn(remoteConn, ctx, metadata.TLSFragment, metadata.TLSRecordFragment, metadata.TLSFragmentFallbackDelay)
	}
	if metadata.TLSSpoof != "" {
		spoofConn, spoofErr := tlsspoof.NewConn(remoteConn, metadata.TLSSpoofMethod, metadata.TLSSpoof)
		if spoofErr != nil {
			spoofErr = E.Cause(spoofErr, "tls_spoof setup")
			remoteConn.Close()
			N.CloseOnHandshakeFailure(conn, onClose, spoofErr)
			m.logger.ErrorContext(ctx, spoofErr)
			return
		}
		remoteConn = spoofConn
	}
	serverFirst := sniff.Skip(&metadata)
	var done atomic.Bool
	if m.kickWriteHandshake(ctx, conn, remoteConn, serverFirst, false, &done, onClose) {
		return
	}
	if m.kickWriteHandshake(ctx, remoteConn, conn, serverFirst, true, &done, onClose) {
		return
	}
	go m.connectionCopy(ctx, conn, remoteConn, false, &done, onClose)
	go m.connectionCopy(ctx, remoteConn, conn, true, &done, onClose)
}

func (m *ConnectionManager) NewPacketConnection(ctx context.Context, this N.Dialer, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = adapter.WithContext(ctx, &metadata)
	var (
		remotePacketConn   net.PacketConn
		remoteConn         net.Conn
		destinationAddress netip.Addr
		err                error
	)
	if metadata.UDPConnect {
		parallelDialer, isParallelDialer := this.(dialer.ParallelInterfaceDialer)
		if len(metadata.DestinationAddresses) > 0 {
			if isParallelDialer {
				remoteConn, err = dialer.DialSerialNetwork(ctx, parallelDialer, N.NetworkUDP, metadata.Destination, metadata.DestinationAddresses, metadata.NetworkStrategy, metadata.NetworkType, metadata.FallbackNetworkType, metadata.FallbackDelay)
			} else {
				remoteConn, err = N.DialSerial(ctx, this, N.NetworkUDP, metadata.Destination, metadata.DestinationAddresses)
			}
		} else if metadata.Destination.IsIP() {
			if isParallelDialer {
				remoteConn, err = dialer.DialSerialNetwork(ctx, parallelDialer, N.NetworkUDP, metadata.Destination, metadata.DestinationAddresses, metadata.NetworkStrategy, metadata.NetworkType, metadata.FallbackNetworkType, metadata.FallbackDelay)
			} else {
				remoteConn, err = this.DialContext(ctx, N.NetworkUDP, metadata.Destination)
			}
		} else {
			remoteConn, err = this.DialContext(ctx, N.NetworkUDP, metadata.Destination)
		}
		if err != nil {
			var remoteString string
			if len(metadata.DestinationAddresses) > 0 {
				remoteString = "[" + strings.Join(common.Map(metadata.DestinationAddresses, netip.Addr.String), ",") + "]"
			} else {
				remoteString = metadata.Destination.String()
			}
			var dialerString string
			if outbound, isOutbound := this.(adapter.Outbound); isOutbound {
				dialerString = " using outbound/" + outbound.Type() + "[" + outbound.Tag() + "]"
			}
			err = E.Cause(err, "open packet connection to ", remoteString, dialerString)
			N.CloseOnHandshakeFailure(conn, onClose, err)
			m.logger.ErrorContext(ctx, err)
			return
		}
		remotePacketConn = bufio.NewUnbindPacketConn(remoteConn)
		connRemoteAddr := M.AddrFromNet(remoteConn.RemoteAddr())
		if connRemoteAddr != metadata.Destination.Addr {
			destinationAddress = connRemoteAddr
		}
	} else {
		if len(metadata.DestinationAddresses) > 0 {
			remotePacketConn, destinationAddress, err = dialer.ListenSerialNetworkPacket(ctx, this, metadata.Destination, metadata.DestinationAddresses, metadata.NetworkStrategy, metadata.NetworkType, metadata.FallbackNetworkType, metadata.FallbackDelay)
		} else if packetDialer, withDestination := this.(dialer.PacketDialerWithDestination); withDestination {
			remotePacketConn, destinationAddress, err = packetDialer.ListenPacketWithDestination(ctx, metadata.Destination)
		} else {
			remotePacketConn, err = this.ListenPacket(ctx, metadata.Destination)
		}
		if err != nil {
			var dialerString string
			if outbound, isOutbound := this.(adapter.Outbound); isOutbound {
				dialerString = " using outbound/" + outbound.Type() + "[" + outbound.Tag() + "]"
			}
			err = E.Cause(err, "listen packet connection using ", dialerString)
			N.CloseOnHandshakeFailure(conn, onClose, err)
			m.logger.ErrorContext(ctx, err)
			return
		}
	}
	err = N.ReportPacketConnHandshakeSuccess(conn, remotePacketConn)
	if err != nil {
		conn.Close()
		remotePacketConn.Close()
		m.logger.ErrorContext(ctx, "report handshake success: ", err)
		return
	}
	udpTimeout := packetTimeout(&metadata)
	var (
		spliceRemote any = remotePacketConn
		spliced      bool
	)
	if remoteConn != nil {
		spliceRemote = remoteConn
	}
	conn, spliced = m.splicePacketConnection(ctx, conn, spliceRemote, &metadata, destinationAddress, udpTimeout, onClose)
	if spliced {
		return
	}
	if destinationAddress.IsValid() {
		var originDestination M.Socksaddr
		if metadata.RouteOriginalDestination.IsValid() {
			originDestination = metadata.RouteOriginalDestination
		} else {
			originDestination = metadata.Destination
		}
		if natConn, loaded := common.Cast[bufio.NATPacketConn](conn); loaded {
			natConn.UpdateDestination(destinationAddress)
		} else {
			destination := M.SocksaddrFrom(destinationAddress, metadata.Destination.Port)
			if metadata.Destination != destination {
				if metadata.UDPDisableDomainUnmapping {
					remotePacketConn = bufio.NewUnidirectionalNATPacketConn(bufio.NewPacketConn(remotePacketConn), destination, originDestination)
				} else {
					remotePacketConn = bufio.NewNATPacketConn(bufio.NewPacketConn(remotePacketConn), destination, originDestination)
				}
			} else if metadata.RouteOriginalDestination.IsValid() && metadata.RouteOriginalDestination != metadata.Destination {
				remotePacketConn = bufio.NewDestinationNATPacketConn(bufio.NewPacketConn(remotePacketConn), metadata.Destination, metadata.RouteOriginalDestination)
			}
		}
	} else if metadata.RouteOriginalDestination.IsValid() && metadata.RouteOriginalDestination != metadata.Destination {
		remotePacketConn = bufio.NewDestinationNATPacketConn(bufio.NewPacketConn(remotePacketConn), metadata.Destination, metadata.RouteOriginalDestination)
	}
	if udpTimeout > 0 {
		ctx, conn = canceler.NewPacketConn(ctx, conn, udpTimeout)
	}
	destination := bufio.NewPacketConn(remotePacketConn)
	var done atomic.Bool
	go m.packetConnectionCopy(ctx, conn, destination, false, &done, onClose)
	go m.packetConnectionCopy(ctx, destination, conn, true, &done, onClose)
}

func (m *ConnectionManager) connectionCopy(ctx context.Context, source net.Conn, destination net.Conn, direction bool, done *atomic.Bool, onClose N.CloseHandlerFunc) {
	_, err := bufio.CopyWithIncreateBuffer(destination, source, bufio.DefaultIncreaseBufferAfter, bufio.DefaultBatchSize)
	if err != nil {
		common.Close(source, destination)
	} else if duplexDst, isDuplex := destination.(N.WriteCloser); isDuplex {
		err = duplexDst.CloseWrite()
		if err != nil {
			common.Close(source, destination)
		}
	} else {
		destination.Close()
	}
	if done.Swap(true) {
		if onClose != nil {
			onClose(err)
		}
		common.Close(source, destination)
	}
	if !direction {
		if err == nil {
			m.logger.DebugContext(ctx, "connection upload finished")
		} else if !E.IsClosedOrCanceled(err) {
			m.logger.ErrorContext(ctx, "connection upload closed: ", err)
		} else {
			m.logger.TraceContext(ctx, "connection upload closed")
		}
	} else {
		if err == nil {
			m.logger.DebugContext(ctx, "connection download finished")
		} else if !E.IsClosedOrCanceled(err) {
			m.logger.ErrorContext(ctx, "connection download closed: ", err)
		} else {
			m.logger.TraceContext(ctx, "connection download closed")
		}
	}
}

func (m *ConnectionManager) kickWriteHandshake(ctx context.Context, source net.Conn, destination net.Conn, serverFirst bool, direction bool, done *atomic.Bool, onClose N.CloseHandlerFunc) bool {
	if !N.NeedHandshakeForWrite(destination) {
		return false
	}
	var (
		err          error
		wrotePayload bool
	)
	if serverFirst {
		_ = destination.SetWriteDeadline(time.Now().Add(C.ReadPayloadTimeout))
		_, err = destination.Write(nil)
		_ = destination.SetWriteDeadline(time.Time{})
	} else {
		var cachedBuffer *buf.Buffer
		sourceReader, readCounters := N.UnwrapCountReader(source, nil)
		destinationWriter, writeCounters := N.UnwrapCountWriter(destination, nil)
		if cachedReader, ok := sourceReader.(N.CachedReader); ok {
			cachedBuffer = cachedReader.ReadCached()
		}
		if cachedBuffer != nil {
			wrotePayload = true
			dataLen := cachedBuffer.Len()
			_, err = destinationWriter.Write(cachedBuffer.Bytes())
			cachedBuffer.Release()
			if err == nil {
				for _, counter := range readCounters {
					counter(int64(dataLen))
				}
				for _, counter := range writeCounters {
					counter(int64(dataLen))
				}
			}
		} else {
			_ = destination.SetWriteDeadline(time.Now().Add(C.ReadPayloadTimeout))
			_, err = destinationWriter.Write(nil)
			_ = destination.SetWriteDeadline(time.Time{})
		}
	}
	if err == nil {
		return false
	}
	if !wrotePayload && (E.IsMulti(err, os.ErrInvalid, context.DeadlineExceeded, io.EOF) || E.IsTimeout(err)) {
		return false
	}
	if !done.Swap(true) {
		if onClose != nil {
			onClose(err)
		}
	}
	common.Close(source, destination)
	if !direction {
		m.logger.ErrorContext(ctx, "connection upload handshake: ", err)
	} else {
		m.logger.ErrorContext(ctx, "connection download handshake: ", err)
	}
	return true
}

func (m *ConnectionManager) packetConnectionCopy(ctx context.Context, source N.PacketReader, destination N.PacketWriter, direction bool, done *atomic.Bool, onClose N.CloseHandlerFunc) {
	_, err := bufio.CopyPacket(destination, source)
	if !direction {
		if err == nil {
			m.logger.DebugContext(ctx, "packet upload finished")
		} else if E.IsClosedOrCanceled(err) {
			m.logger.TraceContext(ctx, "packet upload closed")
		} else {
			m.logger.DebugContext(ctx, "packet upload closed: ", err)
		}
	} else {
		if err == nil {
			m.logger.DebugContext(ctx, "packet download finished")
		} else if E.IsClosedOrCanceled(err) {
			m.logger.TraceContext(ctx, "packet download closed")
		} else {
			m.logger.DebugContext(ctx, "packet download closed: ", err)
		}
	}
	if !done.Swap(true) {
		if onClose != nil {
			onClose(err)
		}
	}
	common.Close(source, destination)
}

type socketOwner struct {
	access   sync.Mutex
	original io.Closer
	owner    io.Closer
	closed   bool
}

func (o *socketOwner) Attach(closer io.Closer) (io.Closer, bool) {
	o.access.Lock()
	defer o.access.Unlock()
	if o.closed || o.owner != nil {
		return nil, false
	}
	o.owner = closer
	return o.original, true
}

func (o *socketOwner) detach() bool {
	o.access.Lock()
	defer o.access.Unlock()
	o.owner = nil
	return o.closed
}

func (o *socketOwner) close() bool {
	o.access.Lock()
	o.closed = true
	owner := o.owner
	o.access.Unlock()
	if owner == nil {
		return false
	}
	owner.Close()
	return true
}

type trackedConn struct {
	net.Conn
	socketOwner
	manager *ConnectionManager
	element *list.Element[io.Closer]
}

func (c *trackedConn) SyscallConn() (syscall.RawConn, error) {
	syscallConn, isSyscallConn := c.Conn.(syscall.Conn)
	if !isSyscallConn {
		return nil, os.ErrInvalid
	}
	return syscallConn.SyscallConn()
}

func (c *trackedConn) Detach() {
	if c.socketOwner.detach() {
		c.Conn.Close()
	}
}

func (c *trackedConn) Close() error {
	c.manager.access.Lock()
	c.manager.connections.Remove(c.element)
	c.manager.access.Unlock()
	if c.socketOwner.close() {
		return nil
	}
	return c.Conn.Close()
}

func (c *trackedConn) Upstream() any {
	return c.Conn
}

func (c *trackedConn) ReaderReplaceable() bool {
	return true
}

func (c *trackedConn) WriterReplaceable() bool {
	return true
}

type trackedPacketConn struct {
	N.NetPacketConn
	socketOwner
	manager *ConnectionManager
	element *list.Element[io.Closer]
}

func (c *trackedPacketConn) SyscallConn() (syscall.RawConn, error) {
	syscallConn, isSyscallConn := c.NetPacketConn.(syscall.Conn)
	if !isSyscallConn {
		return nil, os.ErrInvalid
	}
	return syscallConn.SyscallConn()
}

func (c *trackedPacketConn) Detach() {
	if c.socketOwner.detach() {
		c.NetPacketConn.Close()
	}
}

func (c *trackedPacketConn) Close() error {
	c.manager.access.Lock()
	c.manager.connections.Remove(c.element)
	c.manager.access.Unlock()
	if c.socketOwner.close() {
		return nil
	}
	return c.NetPacketConn.Close()
}

func (c *trackedPacketConn) Upstream() any {
	return c.NetPacketConn
}

func (c *trackedPacketConn) ReaderReplaceable() bool {
	return true
}

func (c *trackedPacketConn) WriterReplaceable() bool {
	return true
}

var (
	_ tun.SpliceSocket = (*trackedConn)(nil)
	_ tun.SpliceSocket = (*trackedPacketConn)(nil)
)
