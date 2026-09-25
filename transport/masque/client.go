package masque

import (
	"context"
	"net/netip"
	"slices"
	"sync"
	"time"

	C "github.com/sagernet/sing-box/constant"
	transportHTTP "github.com/sagernet/sing-box/transport/http"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

const (
	reconnectBackoffInitial = time.Second
	reconnectBackoffMax     = time.Minute
)

type Configuration struct {
	Address          []netip.Prefix
	Routes           []AddressRange
	RoutesAdvertised bool
}

type ClientHandler interface {
	UpdateConfiguration(configuration Configuration) error
	WriteInboundBuffers(packetBuffers []*buf.Buffer) error
	FrontHeadroom() int
}

type ClientOptions struct {
	Context         context.Context
	Logger          logger.ContextLogger
	HTTPClient      *transportHTTP.Client
	Path            string
	AdvertiseRoutes []netip.Prefix
	Handler         ClientHandler
}

type Client struct {
	ctx             context.Context
	cancel          context.CancelFunc
	logger          logger.ContextLogger
	httpClient      *transportHTTP.Client
	template        *Template
	advertiseRoutes []AddressRange
	handler         ClientHandler
	access          sync.Mutex
	current         *clientSession
	suspended       bool
	restarting      bool
	lastError       error
	stateUpdated    chan struct{}
	loopDone        chan struct{}
}

type clientSession struct {
	*session
	client        *Client
	access        sync.Mutex
	configuration Configuration
	ready         bool
}

func NewClient(options ClientOptions) (*Client, error) {
	template, err := ParseTemplate(options.Path)
	if err != nil {
		return nil, err
	}
	advertiseRoutes, err := RangesFromPrefixes(options.AdvertiseRoutes, 0)
	if err != nil {
		return nil, E.Cause(err, "build advertised routes")
	}
	ctx, cancel := context.WithCancel(options.Context)
	return &Client{
		ctx:             ctx,
		cancel:          cancel,
		logger:          options.Logger,
		httpClient:      options.HTTPClient,
		template:        template,
		advertiseRoutes: advertiseRoutes,
		handler:         options.Handler,
		stateUpdated:    make(chan struct{}),
	}, nil
}

func (c *Client) Start() {
	c.loopDone = make(chan struct{})
	go c.loop()
}

func (c *Client) Close() error {
	c.cancel()
	if c.loopDone != nil {
		<-c.loopDone
	}
	return c.httpClient.Close()
}

func (c *Client) notifyStateLocked() {
	close(c.stateUpdated)
	c.stateUpdated = make(chan struct{})
}

func (c *Client) loop() {
	defer close(c.loopDone)
	backoff := reconnectBackoffInitial
	for {
		c.access.Lock()
		suspended := c.suspended
		stateUpdated := c.stateUpdated
		c.restarting = false
		if !suspended {
			c.lastError = nil
		}
		c.access.Unlock()
		if suspended {
			select {
			case <-stateUpdated:
				continue
			case <-c.ctx.Done():
				return
			}
		}
		established, err := c.connect()
		if c.ctx.Err() != nil {
			return
		}
		c.access.Lock()
		interrupted := c.suspended || c.restarting
		if !interrupted {
			c.lastError = err
		}
		c.notifyStateLocked()
		stateUpdated = c.stateUpdated
		c.access.Unlock()
		if interrupted {
			backoff = reconnectBackoffInitial
			continue
		}
		if err != nil {
			c.logger.Error(E.Cause(err, "connection closed"))
		}
		if established {
			backoff = reconnectBackoffInitial
		}
		timer := time.NewTimer(backoff)
		select {
		case <-timer.C:
		case <-stateUpdated:
			timer.Stop()
		case <-c.ctx.Done():
			timer.Stop()
			return
		}
		backoff = min(backoff*2, reconnectBackoffMax)
	}
}

func (c *Client) connect() (bool, error) {
	dialCtx, cancelDial := context.WithTimeout(c.ctx, C.TCPTimeout)
	stream, err := c.httpClient.OpenTunnel(dialCtx, upgradeToken, c.template.Expand())
	cancelDial()
	if err != nil {
		return false, err
	}
	current := &clientSession{client: c}
	c.access.Lock()
	if c.suspended || c.restarting {
		suspended := c.suspended
		c.access.Unlock()
		stream.Close()
		if suspended {
			c.httpClient.ResetConnections()
		}
		return false, nil
	}
	current.session = newSession(c.ctx, stream, current, c.handler.FrontHeadroom)
	c.current = current
	c.access.Unlock()
	err = current.writeCapsule(newAddressCapsule(capsuleTypeAddressRequest, []AssignedAddress{
		{RequestID: 1, Prefix: netip.PrefixFrom(netip.IPv4Unspecified(), 32)},
		{RequestID: 2, Prefix: netip.PrefixFrom(netip.IPv6Unspecified(), 128)},
	}))
	if err == nil && len(c.advertiseRoutes) > 0 {
		err = current.writeCapsule(newRouteCapsule(c.advertiseRoutes))
	}
	if err != nil {
		current.cancel(err)
	}
	err = current.run()
	c.access.Lock()
	c.current = nil
	c.access.Unlock()
	current.access.Lock()
	established := current.ready
	current.ready = false
	current.access.Unlock()
	return established, err
}

func (c *Client) activeSession() *clientSession {
	c.access.Lock()
	defer c.access.Unlock()
	return c.current
}

func (c *Client) Ready() bool {
	current := c.activeSession()
	if current == nil {
		return false
	}
	current.access.Lock()
	defer current.access.Unlock()
	return current.ready
}

func (c *Client) WaitReady(ctx context.Context) error {
	for {
		c.access.Lock()
		current := c.current
		lastError := c.lastError
		stateUpdated := c.stateUpdated
		c.access.Unlock()
		if current != nil {
			current.access.Lock()
			ready := current.ready
			current.access.Unlock()
			if ready {
				return nil
			}
		} else if lastError != nil {
			return lastError
		}
		select {
		case <-stateUpdated:
		case <-ctx.Done():
			return ctx.Err()
		case <-c.ctx.Done():
			return c.ctx.Err()
		}
	}
}

func (c *Client) Suspend() {
	c.access.Lock()
	if c.suspended {
		c.access.Unlock()
		return
	}
	c.suspended = true
	if c.current != nil {
		c.current.cancel(E.New("suspended"))
	}
	c.notifyStateLocked()
	c.access.Unlock()
	c.httpClient.ResetConnections()
}

func (c *Client) Resume() {
	c.access.Lock()
	defer c.access.Unlock()
	if !c.suspended {
		return
	}
	c.suspended = false
	c.lastError = nil
	c.notifyStateLocked()
}

func (c *Client) RestartSession() {
	c.access.Lock()
	c.restarting = true
	if c.current != nil {
		c.current.cancel(E.New("network changed"))
	}
	c.lastError = nil
	c.notifyStateLocked()
	c.access.Unlock()
	c.httpClient.ResetConnections()
}

func (c *Client) WritePacketBuffers(packetBuffers []*buf.Buffer, forwarded bool) error {
	current := c.activeSession()
	if current == nil {
		buf.ReleaseMulti(packetBuffers)
		return nil
	}
	current.access.Lock()
	configuration := current.configuration
	ready := current.ready
	current.access.Unlock()
	if !ready {
		buf.ReleaseMulti(packetBuffers)
		return nil
	}
	inet4Address, inet6Address := firstAddresses(configuration.Address)
	var replies []*buf.Buffer
	routedBuffers := packetBuffers[:0]
	for _, packetBuffer := range packetBuffers {
		_, destination, protocol, valid := packetAddresses(packetBuffer.Bytes())
		if !valid {
			packetBuffer.Release()
			continue
		}
		errorType := tun.ICMPErrorNoRoute
		routed := !configuration.RoutesAdvertised || RoutesContain(configuration.Routes, destination, protocol)
		if routed && forwarded && !decrementHopLimit(packetBuffer.Bytes()) {
			routed = false
			errorType = tun.ICMPErrorHopLimitExceeded
		}
		if !routed {
			reply, built := buildICMPError(packetBuffer.Bytes(), errorType, inet4Address, inet6Address, 0, c.handler.FrontHeadroom())
			if built {
				replies = append(replies, reply)
			}
			packetBuffer.Release()
			continue
		}
		routedBuffers = append(routedBuffers, packetBuffer)
	}
	err := current.writePackets(routedBuffers)
	if err != nil {
		current.cancel(err)
	}
	if len(replies) > 0 {
		return c.handler.WriteInboundBuffers(replies)
	}
	return nil
}

func (s *clientSession) handleAddressAssign(addresses []AssignedAddress) error {
	var assigned []netip.Prefix
	for _, address := range addresses {
		if address.RequestID != 0 && address.Prefix.Addr().IsUnspecified() && address.Prefix.IsSingleIP() {
			continue
		}
		localAddress := address.Prefix.Addr()
		if !address.Prefix.IsSingleIP() {
			localAddress = localAddress.Next()
		}
		assigned = append(assigned, netip.PrefixFrom(localAddress, address.Prefix.Bits()))
	}
	s.access.Lock()
	if slices.Equal(s.configuration.Address, assigned) {
		s.access.Unlock()
		return nil
	}
	s.configuration.Address = assigned
	configuration := s.configuration
	s.access.Unlock()
	return s.updateConfiguration(configuration)
}

func (s *clientSession) handleRouteAdvertisement(routes []AddressRange) error {
	s.access.Lock()
	if s.configuration.RoutesAdvertised && slices.Equal(s.configuration.Routes, routes) {
		s.access.Unlock()
		return nil
	}
	s.configuration.Routes = routes
	s.configuration.RoutesAdvertised = true
	configuration := s.configuration
	s.access.Unlock()
	if len(configuration.Address) == 0 {
		return nil
	}
	return s.updateConfiguration(configuration)
}

func (s *clientSession) updateConfiguration(configuration Configuration) error {
	if len(configuration.Address) == 0 {
		s.access.Lock()
		s.ready = false
		s.access.Unlock()
		s.client.access.Lock()
		s.client.notifyStateLocked()
		s.client.access.Unlock()
		return nil
	}
	err := s.client.handler.UpdateConfiguration(configuration)
	if err != nil {
		return E.Cause(err, "update configuration")
	}
	s.access.Lock()
	s.ready = true
	s.access.Unlock()
	s.client.access.Lock()
	s.client.lastError = nil
	s.client.notifyStateLocked()
	s.client.access.Unlock()
	return nil
}

func (s *clientSession) handleAddressRequest(addresses []AssignedAddress) error {
	rejections := make([]AssignedAddress, 0, len(addresses))
	for _, address := range addresses {
		unspecified := netip.IPv4Unspecified()
		if address.Prefix.Addr().Is6() {
			unspecified = netip.IPv6Unspecified()
		}
		rejections = append(rejections, AssignedAddress{RequestID: address.RequestID, Prefix: netip.PrefixFrom(unspecified, unspecified.BitLen())})
	}
	return s.writeCapsule(newAddressCapsule(capsuleTypeAddressAssign, rejections))
}

func (s *clientSession) handlePacket(buffer *buf.Buffer) {
	_, destination, _, valid := packetAddresses(buffer.Bytes())
	if !valid {
		buffer.Release()
		return
	}
	s.access.Lock()
	configuration := s.configuration
	s.access.Unlock()
	if !prefixesContain(configuration.Address, destination) && !rangesContain(s.client.advertiseRoutes, destination) {
		inet4Address, inet6Address := firstAddresses(configuration.Address)
		reply, built := buildICMPError(buffer.Bytes(), tun.ICMPErrorNoRoute, inet4Address, inet6Address, 0, transportHTTP.CapsuleHeadroom)
		buffer.Release()
		if built {
			_ = s.writePackets([]*buf.Buffer{reply})
		}
		return
	}
	err := s.client.handler.WriteInboundBuffers([]*buf.Buffer{buffer})
	if err != nil {
		s.client.logger.Debug(E.Cause(err, "write packet to device"))
	}
}

func (s *clientSession) handlePacketTooBig(buffer *buf.Buffer, mtu int) {
	s.access.Lock()
	inet4Address, inet6Address := firstAddresses(s.configuration.Address)
	s.access.Unlock()
	reply, built := buildICMPError(buffer.Bytes(), tun.ICMPErrorPacketTooBig, inet4Address, inet6Address, mtu, s.client.handler.FrontHeadroom())
	buffer.Release()
	if !built {
		return
	}
	err := s.client.handler.WriteInboundBuffers([]*buf.Buffer{reply})
	if err != nil {
		s.client.logger.Debug(E.Cause(err, "write packet to device"))
	}
}

func firstAddresses(addresses []netip.Prefix) (netip.Addr, netip.Addr) {
	var inet4Address netip.Addr
	var inet6Address netip.Addr
	for _, prefix := range addresses {
		if prefix.Addr().Is4() && !inet4Address.IsValid() {
			inet4Address = prefix.Addr()
		} else if prefix.Addr().Is6() && !inet6Address.IsValid() {
			inet6Address = prefix.Addr()
		}
	}
	return inet4Address, inet6Address
}

func prefixesContain(prefixes []netip.Prefix, address netip.Addr) bool {
	return slices.ContainsFunc(prefixes, func(prefix netip.Prefix) bool {
		return prefix.Contains(address)
	})
}

func RoutesContain(routes []AddressRange, address netip.Addr, protocol uint8) bool {
	return slices.ContainsFunc(routes, func(route AddressRange) bool {
		return route.Contains(address) && (route.Protocol == 0 || route.Protocol == protocol || isControlProtocol(protocol))
	})
}

func rangesContain(routes []AddressRange, address netip.Addr) bool {
	return slices.ContainsFunc(routes, func(route AddressRange) bool {
		return route.Contains(address)
	})
}
