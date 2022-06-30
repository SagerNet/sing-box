package adapter

import (
	"context"
	"net"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/database64128/tfo-go"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type InboundHandler interface {
	Type() string
	Network() []string
	NewConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata InboundContext) error
}

var _ Inbound = (*DefaultInboundService)(nil)

type DefaultInboundService struct {
	ctx         context.Context
	logger      log.Logger
	tag         string
	listen      netip.AddrPort
	listenerTFO bool
	handler     InboundHandler
	tcpListener *net.TCPListener
	udpConn     *net.UDPConn
	forceAddr6  bool
	access      sync.RWMutex
	closed      chan struct{}
	outbound    chan *defaultInboundUDPServiceOutboundPacket
}

func NewDefaultInboundService(ctx context.Context, tag string, logger log.Logger, listen netip.AddrPort, listenerTFO bool, handler InboundHandler) *DefaultInboundService {
	return &DefaultInboundService{
		ctx:         ctx,
		logger:      logger,
		tag:         tag,
		listen:      listen,
		listenerTFO: listenerTFO,
		handler:     handler,
		closed:      make(chan struct{}),
		outbound:    make(chan *defaultInboundUDPServiceOutboundPacket),
	}
}

func (s *DefaultInboundService) Type() string {
	return s.handler.Type()
}

func (s *DefaultInboundService) Tag() string {
	return s.tag
}

func (s *DefaultInboundService) Start() error {
	var listenAddr net.Addr
	if common.Contains(s.handler.Network(), C.NetworkTCP) {
		var tcpListener *net.TCPListener
		var err error
		if !s.listenerTFO {
			tcpListener, err = net.ListenTCP(M.NetworkFromNetAddr("tcp", s.listen.Addr()), M.SocksaddrFromNetIP(s.listen).TCPAddr())
		} else {
			tcpListener, err = tfo.ListenTCP(M.NetworkFromNetAddr("tcp", s.listen.Addr()), M.SocksaddrFromNetIP(s.listen).TCPAddr())
		}
		if err != nil {
			return err
		}
		s.tcpListener = tcpListener
		go s.loopTCPIn()
		listenAddr = tcpListener.Addr()
	}
	if common.Contains(s.handler.Network(), C.NetworkUDP) {
		udpConn, err := net.ListenUDP(M.NetworkFromNetAddr("udp", s.listen.Addr()), M.SocksaddrFromNetIP(s.listen).UDPAddr())
		if err != nil {
			return err
		}
		s.udpConn = udpConn
		s.forceAddr6 = M.SocksaddrFromNet(udpConn.LocalAddr()).Addr.Is6()
		if _, threadUnsafeHandler := common.Cast[N.ThreadUnsafeWriter](s.handler); !threadUnsafeHandler {
			go s.loopUDPIn()
		} else {
			go s.loopUDPInThreadSafe()
		}
		go s.loopUDPOut()
		if listenAddr == nil {
			listenAddr = udpConn.LocalAddr()
		}
	}
	s.logger.Info("server started at ", listenAddr)
	return nil
}

func (s *DefaultInboundService) Close() error {
	return common.Close(
		common.PtrOrNil(s.tcpListener),
		common.PtrOrNil(s.udpConn),
	)
}

func (s *DefaultInboundService) Upstream() any {
	return s.handler
}

func (s *DefaultInboundService) loopTCPIn() {
	tcpListener := s.tcpListener
	for {
		conn, err := tcpListener.Accept()
		if err != nil {
			return
		}
		var metadata InboundContext
		metadata.Inbound = s.tag
		metadata.Source = M.AddrPortFromNet(conn.RemoteAddr())
		go func() {
			metadata.Network = "tcp"
			ctx := log.ContextWithID(s.ctx)
			s.logger.WithContext(ctx).Info("inbound connection from ", conn.RemoteAddr())
			hErr := s.handler.NewConnection(ctx, conn, metadata)
			if hErr != nil {
				s.newContextError(ctx, E.Cause(hErr, "process connection from ", conn.RemoteAddr()))
			}
		}()
	}
}

func (s *DefaultInboundService) loopUDPIn() {
	defer close(s.closed)
	_buffer := buf.StackNewPacket()
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	buffer.IncRef()
	defer buffer.DecRef()
	packetService := (*defaultInboundUDPService)(s)
	var metadata InboundContext
	metadata.Inbound = s.tag
	metadata.Network = "udp"
	for {
		buffer.Reset()
		n, addr, err := s.udpConn.ReadFromUDPAddrPort(buffer.FreeBytes())
		if err != nil {
			return
		}
		buffer.Truncate(n)
		metadata.Source = addr
		err = s.handler.NewPacket(s.ctx, packetService, buffer, metadata)
		if err != nil {
			s.newError(E.Cause(err, "process packet from ", addr))
		}
	}
}

func (s *DefaultInboundService) loopUDPInThreadSafe() {
	defer close(s.closed)
	packetService := (*defaultInboundUDPService)(s)
	var metadata InboundContext
	metadata.Inbound = s.tag
	metadata.Network = "udp"
	for {
		buffer := buf.NewPacket()
		n, addr, err := s.udpConn.ReadFromUDPAddrPort(buffer.FreeBytes())
		if err != nil {
			return
		}
		buffer.Truncate(n)
		metadata.Source = addr
		err = s.handler.NewPacket(s.ctx, packetService, buffer, metadata)
		if err != nil {
			buffer.Release()
			s.newError(E.Cause(err, "process packet from ", addr))
		}
	}
}

func (s *DefaultInboundService) loopUDPOut() {
	for {
		select {
		case packet := <-s.outbound:
			err := s.writePacket(packet.buffer, packet.destination)
			if err != nil && !E.IsClosed(err) {
				s.newError(E.New("write back udp: ", err))
			}
			continue
		case <-s.closed:
		}
		for {
			select {
			case packet := <-s.outbound:
				packet.buffer.Release()
			default:
				return
			}
		}
	}
}

func (s *DefaultInboundService) newError(err error) {
	s.logger.Warn(err)
}

func (s *DefaultInboundService) newContextError(ctx context.Context, err error) {
	common.Close(err)
	if E.IsClosed(err) {
		s.logger.WithContext(ctx).Debug("connection closed")
		return
	}
	s.logger.Error(err)
}

func (s *DefaultInboundService) writePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	if destination.Family().IsFqdn() {
		udpAddr, err := net.ResolveUDPAddr("udp", destination.String())
		if err != nil {
			return err
		}
		return common.Error(s.udpConn.WriteTo(buffer.Bytes(), udpAddr))
	}
	if s.forceAddr6 && destination.Addr.Is4() {
		destination.Addr = netip.AddrFrom16(destination.Addr.As16())
	}
	return common.Error(s.udpConn.WriteToUDPAddrPort(buffer.Bytes(), destination.AddrPort()))
}

type defaultInboundUDPService DefaultInboundService

func (s *defaultInboundUDPService) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	n, addr, err := s.udpConn.ReadFromUDPAddrPort(buffer.FreeBytes())
	if err != nil {
		return M.Socksaddr{}, err
	}
	buffer.Truncate(n)
	return M.SocksaddrFromNetIP(addr), nil
}

func (s *defaultInboundUDPService) WriteIsThreadUnsafe() {
}

type defaultInboundUDPServiceOutboundPacket struct {
	buffer      *buf.Buffer
	destination M.Socksaddr
}

func (s *defaultInboundUDPService) Upstream() any {
	return s.udpConn
}

func (s *defaultInboundUDPService) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	s.access.RLock()
	defer s.access.RUnlock()

	select {
	case <-s.closed:
		return os.ErrClosed
	default:
	}

	s.outbound <- &defaultInboundUDPServiceOutboundPacket{buffer, destination}
	return nil
}

func (s *defaultInboundUDPService) Close() error {
	return s.udpConn.Close()
}

func (s *defaultInboundUDPService) LocalAddr() net.Addr {
	return s.udpConn.LocalAddr()
}

func (s *defaultInboundUDPService) SetDeadline(t time.Time) error {
	return s.udpConn.SetDeadline(t)
}

func (s *defaultInboundUDPService) SetReadDeadline(t time.Time) error {
	return s.udpConn.SetReadDeadline(t)
}

func (s *defaultInboundUDPService) SetWriteDeadline(t time.Time) error {
	return s.udpConn.SetWriteDeadline(t)
}
