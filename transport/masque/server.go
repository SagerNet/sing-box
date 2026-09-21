package masque

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"sync"

	transportHTTP "github.com/sagernet/sing-box/transport/http"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	"go4.org/netipx"
)

type ServerHandler interface {
	WriteInboundBuffers(packetBuffers []*buf.Buffer) error
}

type ServerOptions struct {
	Context         context.Context
	Logger          logger.ContextLogger
	Path            string
	Address         []netip.Prefix
	AdvertiseRoutes []netip.Prefix
	Resolve         func(ctx context.Context, domain string) ([]netip.Addr, error)
	Handler         ServerHandler
}

type Server struct {
	ctx             context.Context
	logger          logger.ContextLogger
	template        *Template
	pools           []*addressPool
	inet4Address    netip.Addr
	inet6Address    netip.Addr
	advertiseRoutes *netipx.IPSet
	resolve         func(ctx context.Context, domain string) ([]netip.Addr, error)
	handler         ServerHandler
	access          sync.RWMutex
	addresses       map[netip.Addr]*serverSession
	advertisements  []*serverSession
}

type serverSession struct {
	*session
	server           *Server
	ctx              context.Context
	user             string
	addresses        []netip.Addr
	advertisedRoutes []AddressRange
	peerRoutes       []AddressRange
}

func NewServer(options ServerOptions) (*Server, error) {
	template, err := ParseTemplate(options.Path)
	if err != nil {
		return nil, err
	}
	if len(options.Address) == 0 {
		return nil, E.New("missing address")
	}
	server := &Server{
		ctx:       options.Context,
		logger:    options.Logger,
		template:  template,
		resolve:   options.Resolve,
		handler:   options.Handler,
		addresses: make(map[netip.Addr]*serverSession),
	}
	var builder netipx.IPSetBuilder
	for _, address := range options.Address {
		builder.AddPrefix(address.Masked())
		if address.Addr().Is4() {
			if server.inet4Address.IsValid() {
				return nil, E.New("multiple IPv4 addresses are not supported")
			}
			server.inet4Address = address.Addr()
		} else {
			if server.inet6Address.IsValid() {
				return nil, E.New("multiple IPv6 addresses are not supported")
			}
			server.inet6Address = address.Addr()
		}
		pool := newAddressPool(address)
		probe, allocated := pool.allocate()
		if !allocated {
			return nil, E.New("address ", address, " leaves no addresses to assign")
		}
		pool.release(probe)
		pool.next = pool.prefix.Addr()
		server.pools = append(server.pools, pool)
	}
	if len(options.AdvertiseRoutes) == 0 {
		builder.AddPrefix(netip.PrefixFrom(netip.IPv4Unspecified(), 0))
		builder.AddPrefix(netip.PrefixFrom(netip.IPv6Unspecified(), 0))
	}
	for _, route := range options.AdvertiseRoutes {
		builder.AddPrefix(route)
	}
	server.advertiseRoutes, err = builder.IPSet()
	if err != nil {
		return nil, E.Cause(err, "build advertised routes")
	}
	return server, nil
}

func (s *Server) NewTunnelRequest(ctx context.Context, request transportHTTP.TunnelRequest) {
	scope, matched, err := s.template.Match(request.Request().URL)
	if !matched {
		s.logger.ErrorContext(ctx, "process connection from ", request.Source(), ": unexpected path: ", request.Request().URL.Path)
		request.Reject(http.StatusNotFound, nil)
		return
	}
	if err != nil {
		s.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", request.Source()))
		request.Reject(http.StatusBadRequest, nil)
		return
	}
	var builder netipx.IPSetBuilder
	switch {
	case scope.Domain != "":
		addresses, resolveErr := s.resolve(ctx, scope.Domain)
		if resolveErr != nil {
			s.logger.ErrorContext(ctx, E.Cause(resolveErr, "process connection from ", request.Source(), ": resolve ", scope.Domain))
			request.Reject(http.StatusBadGateway, http.Header{"Proxy-Status": []string{"sing-box; error=dns_error"}})
			return
		}
		for _, address := range addresses {
			builder.Add(address)
		}
		builder.Intersect(s.advertiseRoutes)
	case scope.Prefix.IsValid():
		builder.AddPrefix(scope.Prefix)
		builder.Intersect(s.advertiseRoutes)
	default:
		builder.AddSet(s.advertiseRoutes)
	}
	current := &serverSession{
		server: s,
		ctx:    ctx,
	}
	current.user, _ = auth.UserFromContext[string](ctx)
	for _, pool := range s.pools {
		address, allocated := pool.allocate()
		if !allocated {
			continue
		}
		current.addresses = append(current.addresses, address)
	}
	defer s.releaseSession(current)
	if len(current.addresses) == 0 {
		s.logger.ErrorContext(ctx, "process connection from ", request.Source(), ": address pool exhausted")
		request.Reject(http.StatusServiceUnavailable, nil)
		return
	}
	var familyBuilder netipx.IPSetBuilder
	for _, address := range current.addresses {
		if address.Is4() {
			familyBuilder.AddPrefix(netip.PrefixFrom(netip.IPv4Unspecified(), 0))
		} else {
			familyBuilder.AddPrefix(netip.PrefixFrom(netip.IPv6Unspecified(), 0))
		}
	}
	families, _ := familyBuilder.IPSet()
	builder.Intersect(families)
	routeSet, err := builder.IPSet()
	if err != nil {
		s.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", request.Source(), ": build routes"))
		request.Reject(http.StatusInternalServerError, nil)
		return
	}
	current.advertisedRoutes = common.Map(routeSet.Ranges(), func(it netipx.IPRange) AddressRange {
		return AddressRange{Start: it.From(), End: it.To(), Protocol: scope.Protocol}
	})
	if len(current.advertisedRoutes) == 0 {
		s.logger.ErrorContext(ctx, "process connection from ", request.Source(), ": requested target is not routable")
		request.Reject(http.StatusForbidden, nil)
		return
	}
	stream, err := request.Accept()
	if err != nil {
		s.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", request.Source()))
		return
	}
	current.session = newSession(s.ctx, stream, current, true)
	s.access.Lock()
	for _, address := range current.addresses {
		s.addresses[address] = current
	}
	s.access.Unlock()
	assignedAddresses := strings.Join(common.Map(current.addresses, netip.Addr.String), " ")
	if current.user != "" {
		s.logger.InfoContext(ctx, "[", current.user, "] inbound tunnel from ", request.Source(), " assigned ", assignedAddresses)
	} else {
		s.logger.InfoContext(ctx, "inbound tunnel from ", request.Source(), " assigned ", assignedAddresses)
	}
	err = current.writeCapsule(newAddressCapsule(capsuleTypeAddressAssign, current.assignedAddresses(nil)))
	if err == nil {
		err = current.writeCapsule(newRouteCapsule(current.advertisedRoutes))
	}
	if err != nil {
		current.cancel(err)
	}
	err = current.run()
	if err != nil && !E.IsClosedOrCanceled(err) {
		s.logger.ErrorContext(ctx, E.Cause(err, "tunnel from ", request.Source(), " closed"))
	} else {
		s.logger.DebugContext(ctx, "tunnel from ", request.Source(), " closed")
	}
}

func (s *Server) releaseSession(current *serverSession) {
	s.access.Lock()
	for _, address := range current.addresses {
		if s.addresses[address] == current {
			delete(s.addresses, address)
		}
	}
	s.advertisements = slices.DeleteFunc(s.advertisements, func(it *serverSession) bool {
		return it == current
	})
	s.access.Unlock()
	for _, address := range current.addresses {
		for _, pool := range s.pools {
			if pool.prefix.Contains(address) {
				pool.release(address)
			}
		}
	}
}

func (s *Server) lookup(destination netip.Addr, protocol uint8) *serverSession {
	s.access.RLock()
	defer s.access.RUnlock()
	current, loaded := s.addresses[destination]
	if loaded {
		return current
	}
	if destination == s.inet4Address || destination == s.inet6Address {
		return nil
	}
	for _, advertisement := range slices.Backward(s.advertisements) {
		if RoutesContain(advertisement.peerRoutes, destination, protocol) {
			return advertisement
		}
	}
	return nil
}

func (s *Server) Contains(address netip.Addr) bool {
	s.access.RLock()
	defer s.access.RUnlock()
	_, loaded := s.addresses[address]
	return loaded || slices.ContainsFunc(s.advertisements, func(it *serverSession) bool {
		return rangesContain(it.peerRoutes, address)
	})
}

func (s *Server) WritePacketBuffers(packetBuffers []*buf.Buffer, forwarded bool) error {
	var replies []*buf.Buffer
	for _, packetBuffer := range packetBuffers {
		source, destination, protocol, valid := packetAddresses(packetBuffer.Bytes())
		if !valid {
			packetBuffer.Release()
			continue
		}
		errorType := tun.ICMPErrorNoRoute
		target := s.lookup(destination, protocol)
		routed := target != nil && target.accepts(source, protocol)
		if routed && forwarded && !decrementHopLimit(packetBuffer.Bytes()) {
			routed = false
			errorType = tun.ICMPErrorHopLimitExceeded
		}
		if routed {
			buffer := buf.NewSize(transportHTTP.CapsuleHeadroom + packetBuffer.Len())
			buffer.Resize(transportHTTP.CapsuleHeadroom, 0)
			buffer.Write(packetBuffer.Bytes())
			target.queuePacket(buffer)
		} else {
			reply, built := buildICMPError(packetBuffer.Bytes(), errorType, s.inet4Address, s.inet6Address, 0, PacketHeadroom)
			if built {
				replies = append(replies, reply)
			}
		}
		packetBuffer.Release()
	}
	if len(replies) > 0 {
		return s.handler.WriteInboundBuffers(replies)
	}
	return nil
}

func (s *Server) Close() error {
	s.access.RLock()
	sessions := make([]*serverSession, 0, len(s.addresses))
	for _, current := range s.addresses {
		sessions = append(sessions, current)
	}
	s.access.RUnlock()
	for _, current := range sessions {
		current.cancel(net.ErrClosed)
	}
	return nil
}

func (s *serverSession) accepts(source netip.Addr, protocol uint8) bool {
	return isControlProtocol(protocol) || RoutesContain(s.advertisedRoutes, source, protocol)
}

func (s *serverSession) assignedAddresses(requests []AssignedAddress) []AssignedAddress {
	assigned := make([]AssignedAddress, 0, len(s.addresses)+len(requests))
	answered := make(map[netip.Addr]bool)
	for _, request := range requests {
		index := slices.IndexFunc(s.addresses, func(address netip.Addr) bool {
			return address.Is4() == request.Prefix.Addr().Is4() && !answered[address]
		})
		if index == -1 {
			unspecified := netip.IPv4Unspecified()
			if request.Prefix.Addr().Is6() {
				unspecified = netip.IPv6Unspecified()
			}
			assigned = append(assigned, AssignedAddress{RequestID: request.RequestID, Prefix: netip.PrefixFrom(unspecified, unspecified.BitLen())})
			continue
		}
		address := s.addresses[index]
		answered[address] = true
		assigned = append(assigned, AssignedAddress{RequestID: request.RequestID, Prefix: netip.PrefixFrom(address, address.BitLen())})
	}
	for _, address := range s.addresses {
		if !answered[address] {
			assigned = append(assigned, AssignedAddress{Prefix: netip.PrefixFrom(address, address.BitLen())})
		}
	}
	return assigned
}

func (s *serverSession) handleAddressAssign(addresses []AssignedAddress) error {
	return nil
}

func (s *serverSession) handleAddressRequest(addresses []AssignedAddress) error {
	return s.writeCapsule(newAddressCapsule(capsuleTypeAddressAssign, s.assignedAddresses(addresses)))
}

func (s *serverSession) handleRouteAdvertisement(routes []AddressRange) error {
	s.server.access.Lock()
	s.peerRoutes = routes
	s.server.advertisements = slices.DeleteFunc(s.server.advertisements, func(it *serverSession) bool {
		return it == s
	})
	if len(routes) > 0 {
		s.server.advertisements = append(s.server.advertisements, s)
	}
	s.server.access.Unlock()
	s.server.logger.InfoContext(s.ctx, "peer advertised ", len(routes), " routes")
	return nil
}

func (s *serverSession) handlePacket(buffer *buf.Buffer) {
	source, destination, protocol, valid := packetAddresses(buffer.Bytes())
	if !valid {
		buffer.Release()
		return
	}
	s.server.access.RLock()
	peerRoutes := s.peerRoutes
	s.server.access.RUnlock()
	errorType := tun.ICMPErrorNoRoute
	switch {
	case !slices.Contains(s.addresses, source) && !rangesContain(peerRoutes, source):
		errorType = tun.ICMPErrorSourcePolicy
	case destination.IsLinkLocalUnicast() || destination.IsLinkLocalMulticast() || destination.IsInterfaceLocalMulticast():
		buffer.Release()
		return
	case !RoutesContain(s.advertisedRoutes, destination, protocol):
	default:
		target := s.server.lookup(destination, protocol)
		if target == nil && destination != s.server.inet4Address && destination != s.server.inet6Address && slices.ContainsFunc(s.server.pools, func(pool *addressPool) bool {
			return pool.prefix.Contains(destination)
		}) {
			errorType = tun.ICMPErrorAddressUnreachable
			break
		}
		if target == nil {
			err := s.server.handler.WriteInboundBuffers([]*buf.Buffer{buffer})
			if err != nil {
				s.server.logger.DebugContext(s.ctx, E.Cause(err, "write packet to device"))
			}
			return
		}
		if !target.accepts(source, protocol) {
			break
		}
		if !decrementHopLimit(buffer.Bytes()) {
			errorType = tun.ICMPErrorHopLimitExceeded
			break
		}
		target.queuePacket(buffer)
		return
	}
	reply, built := buildICMPError(buffer.Bytes(), errorType, s.server.inet4Address, s.server.inet6Address, 0, transportHTTP.CapsuleHeadroom)
	buffer.Release()
	if built {
		s.queuePacket(reply)
	}
}

func (s *serverSession) handlePacketTooBig(buffer *buf.Buffer, mtu int) {
	reply, built := buildICMPError(buffer.Bytes(), tun.ICMPErrorPacketTooBig, s.server.inet4Address, s.server.inet6Address, mtu, PacketHeadroom)
	buffer.Release()
	if !built {
		return
	}
	_, destination, protocol, _ := packetAddresses(reply.Bytes())
	target := s.server.lookup(destination, protocol)
	if target != nil {
		target.queuePacket(reply)
		return
	}
	err := s.server.handler.WriteInboundBuffers([]*buf.Buffer{reply})
	if err != nil {
		s.server.logger.DebugContext(s.ctx, E.Cause(err, "write packet to device"))
	}
}
