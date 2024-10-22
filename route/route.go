package route

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"os/user"
	"strings"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/conntrack"
	"github.com/sagernet/sing-box/common/process"
	"github.com/sagernet/sing-box/common/sniff"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/outbound"
	"github.com/sagernet/sing-box/route/rule"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing-mux"
	"github.com/sagernet/sing-vmess"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/bufio/deadline"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
)

// Deprecated: use RouteConnectionEx instead.
func (r *Router) RouteConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return r.routeConnection(ctx, conn, metadata, nil)
}

func (r *Router) RouteConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	err := r.routeConnection(ctx, conn, metadata, onClose)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		if E.IsClosedOrCanceled(err) {
			r.logger.DebugContext(ctx, "connection closed: ", err)
		} else {
			r.logger.ErrorContext(ctx, err)
		}
	}
}

func (r *Router) routeConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) error {
	if r.pauseManager.IsDevicePaused() {
		return E.New("reject connection to ", metadata.Destination, " while device paused")
	}

	if metadata.InboundDetour != "" {
		if metadata.LastInbound == metadata.InboundDetour {
			return E.New("routing loop on detour: ", metadata.InboundDetour)
		}
		detour := r.inboundByTag[metadata.InboundDetour]
		if detour == nil {
			return E.New("inbound detour not found: ", metadata.InboundDetour)
		}
		injectable, isInjectable := detour.(adapter.TCPInjectableInbound)
		if !isInjectable {
			return E.New("inbound detour is not TCP injectable: ", metadata.InboundDetour)
		}
		metadata.LastInbound = metadata.Inbound
		metadata.Inbound = metadata.InboundDetour
		metadata.InboundDetour = ""
		injectable.NewConnectionEx(ctx, conn, metadata, onClose)
		return nil
	}
	conntrack.KillerCheck()
	metadata.Network = N.NetworkTCP
	switch metadata.Destination.Fqdn {
	case mux.Destination.Fqdn:
		return E.New("global multiplex is deprecated since sing-box v1.7.0, enable multiplex in inbound options instead.")
	case vmess.MuxDestination.Fqdn:
		return E.New("global multiplex (v2ray legacy) not supported since sing-box v1.7.0.")
	case uot.MagicAddress:
		return E.New("global UoT not supported since sing-box v1.7.0.")
	case uot.LegacyMagicAddress:
		return E.New("global UoT (legacy) not supported since sing-box v1.7.0.")
	}
	if deadline.NeedAdditionalReadDeadline(conn) {
		conn = deadline.NewConn(conn)
	}
	selectedRule, _, buffers, err := r.matchRule(ctx, &metadata, false, conn, nil, -1)
	if err != nil {
		return err
	}
	var selectedOutbound adapter.Outbound
	var selectReturn bool
	if selectedRule != nil {
		switch action := selectedRule.Action().(type) {
		case *rule.RuleActionRoute:
			var loaded bool
			selectedOutbound, loaded = r.Outbound(action.Outbound)
			if !loaded {
				buf.ReleaseMulti(buffers)
				return E.New("outbound not found: ", action.Outbound)
			}
		case *rule.RuleActionReturn:
			selectReturn = true
		case *rule.RuleActionReject:
			buf.ReleaseMulti(buffers)
			N.CloseOnHandshakeFailure(conn, onClose, action.Error())
			return nil
		}
	}
	if selectedRule == nil || selectReturn {
		if r.defaultOutboundForConnection == nil {
			buf.ReleaseMulti(buffers)
			return E.New("missing default outbound with TCP support")
		}
		selectedOutbound = r.defaultOutboundForConnection
	}
	if !common.Contains(selectedOutbound.Network(), N.NetworkTCP) {
		buf.ReleaseMulti(buffers)
		return E.New("TCP is not supported by outbound: ", selectedOutbound.Tag())
	}
	for _, buffer := range buffers {
		conn = bufio.NewCachedConn(conn, buffer)
	}
	if r.clashServer != nil {
		trackerConn, tracker := r.clashServer.RoutedConnection(ctx, conn, metadata, selectedRule)
		defer tracker.Leave()
		conn = trackerConn
	}
	if r.v2rayServer != nil {
		if statsService := r.v2rayServer.StatsService(); statsService != nil {
			conn = statsService.RoutedConnection(metadata.Inbound, selectedOutbound.Tag(), metadata.User, conn)
		}
	}
	legacyOutbound, isLegacy := selectedOutbound.(adapter.ConnectionHandler)
	if isLegacy {
		err = legacyOutbound.NewConnection(ctx, conn, metadata)
		if err != nil {
			conn.Close()
			if onClose != nil {
				onClose(err)
			}
			return E.Cause(err, "outbound/", selectedOutbound.Type(), "[", selectedOutbound.Tag(), "]")
		} else {
			if onClose != nil {
				onClose(nil)
			}
		}
		return nil
	}
	// TODO
	err = outbound.NewConnection(ctx, selectedOutbound, conn, metadata)
	if err != nil {
		conn.Close()
		if onClose != nil {
			onClose(err)
		}
		return E.Cause(err, "outbound/", selectedOutbound.Type(), "[", selectedOutbound.Tag(), "]")
	} else {
		if onClose != nil {
			onClose(nil)
		}
	}
	return nil
}

func (r *Router) RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	err := r.routePacketConnection(ctx, conn, metadata, nil)
	if err != nil {
		conn.Close()
		if E.IsClosedOrCanceled(err) {
			r.logger.DebugContext(ctx, "connection closed: ", err)
		} else {
			r.logger.ErrorContext(ctx, err)
		}
	}
	return nil
}

func (r *Router) RoutePacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	err := r.routePacketConnection(ctx, conn, metadata, onClose)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		if E.IsClosedOrCanceled(err) {
			r.logger.DebugContext(ctx, "connection closed: ", err)
		} else {
			r.logger.ErrorContext(ctx, err)
		}
	} else if onClose != nil {
		onClose(nil)
	}
}

func (r *Router) routePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) error {
	if r.pauseManager.IsDevicePaused() {
		return E.New("reject packet connection to ", metadata.Destination, " while device paused")
	}
	if metadata.InboundDetour != "" {
		if metadata.LastInbound == metadata.InboundDetour {
			return E.New("routing loop on detour: ", metadata.InboundDetour)
		}
		detour := r.inboundByTag[metadata.InboundDetour]
		if detour == nil {
			return E.New("inbound detour not found: ", metadata.InboundDetour)
		}
		injectable, isInjectable := detour.(adapter.UDPInjectableInbound)
		if !isInjectable {
			return E.New("inbound detour is not UDP injectable: ", metadata.InboundDetour)
		}
		metadata.LastInbound = metadata.Inbound
		metadata.Inbound = metadata.InboundDetour
		metadata.InboundDetour = ""
		injectable.NewPacketConnectionEx(ctx, conn, metadata, onClose)
		return nil
	}
	conntrack.KillerCheck()

	// TODO: move to UoT
	metadata.Network = N.NetworkUDP

	// Currently we don't have deadline usages for UDP connections
	/*if deadline.NeedAdditionalReadDeadline(conn) {
		conn = deadline.NewPacketConn(bufio.NewNetPacketConn(conn))
	}*/

	selectedRule, _, buffers, err := r.matchRule(ctx, &metadata, false, nil, conn, -1)
	if err != nil {
		return err
	}
	var selectedOutbound adapter.Outbound
	var selectReturn bool
	if selectedRule != nil {
		switch action := selectedRule.Action().(type) {
		case *rule.RuleActionRoute:
			var loaded bool
			selectedOutbound, loaded = r.Outbound(action.Outbound)
			if !loaded {
				buf.ReleaseMulti(buffers)
				return E.New("outbound not found: ", action.Outbound)
			}
			metadata.UDPDisableDomainUnmapping = action.UDPDisableDomainUnmapping
		case *rule.RuleActionReturn:
			selectReturn = true
		case *rule.RuleActionReject:
			buf.ReleaseMulti(buffers)
			N.CloseOnHandshakeFailure(conn, onClose, syscall.ECONNREFUSED)
			return nil
		}
	}
	if selectedRule == nil || selectReturn {
		if r.defaultOutboundForPacketConnection == nil {
			buf.ReleaseMulti(buffers)
			return E.New("missing default outbound with UDP support")
		}
		selectedOutbound = r.defaultOutboundForPacketConnection
	}
	if !common.Contains(selectedOutbound.Network(), N.NetworkUDP) {
		buf.ReleaseMulti(buffers)
		return E.New("UDP is not supported by outbound: ", selectedOutbound.Tag())
	}
	for _, buffer := range buffers {
		// TODO: check if metadata.Destination == packet destination
		conn = bufio.NewCachedPacketConn(conn, buffer, metadata.Destination)
	}
	if r.clashServer != nil {
		trackerConn, tracker := r.clashServer.RoutedPacketConnection(ctx, conn, metadata, selectedRule)
		defer tracker.Leave()
		conn = trackerConn
	}
	if r.v2rayServer != nil {
		if statsService := r.v2rayServer.StatsService(); statsService != nil {
			conn = statsService.RoutedPacketConnection(metadata.Inbound, selectedOutbound.Tag(), metadata.User, conn)
		}
	}
	if metadata.FakeIP {
		conn = bufio.NewNATPacketConn(bufio.NewNetPacketConn(conn), metadata.OriginDestination, metadata.Destination)
	}
	legacyOutbound, isLegacy := selectedOutbound.(adapter.PacketConnectionHandler)
	if isLegacy {
		err = legacyOutbound.NewPacketConnection(ctx, conn, metadata)
		N.CloseOnHandshakeFailure(conn, onClose, err)
		if err != nil {
			return E.Cause(err, "outbound/", selectedOutbound.Type(), "[", selectedOutbound.Tag(), "]")
		}
		return nil
	}
	// TODO
	err = outbound.NewPacketConnection(ctx, selectedOutbound, conn, metadata)
	N.CloseOnHandshakeFailure(conn, onClose, err)
	if err != nil {
		return E.Cause(err, "outbound/", selectedOutbound.Type(), "[", selectedOutbound.Tag(), "]")
	}
	return nil
}

func (r *Router) PreMatch(metadata adapter.InboundContext) error {
	selectedRule, _, _, err := r.matchRule(r.ctx, &metadata, true, nil, nil, -1)
	if err != nil {
		return err
	}
	if selectedRule == nil {
		return nil
	}
	rejectAction, isReject := selectedRule.Action().(*rule.RuleActionReject)
	if !isReject {
		return nil
	}
	return rejectAction.Error()
}

func (r *Router) matchRule(
	ctx context.Context, metadata *adapter.InboundContext, preMatch bool,
	inputConn net.Conn, inputPacketConn N.PacketConn, ruleIndex int,
) (selectedRule adapter.Rule, selectedRuleIndex int, buffers []*buf.Buffer, fatalErr error) {
	if r.processSearcher != nil && metadata.ProcessInfo == nil {
		var originDestination netip.AddrPort
		if metadata.OriginDestination.IsValid() {
			originDestination = metadata.OriginDestination.AddrPort()
		} else if metadata.Destination.IsIP() {
			originDestination = metadata.Destination.AddrPort()
		}
		processInfo, fErr := process.FindProcessInfo(r.processSearcher, ctx, metadata.Network, metadata.Source.AddrPort(), originDestination)
		if fErr != nil {
			r.logger.InfoContext(ctx, "failed to search process: ", fErr)
		} else {
			if processInfo.ProcessPath != "" {
				r.logger.InfoContext(ctx, "found process path: ", processInfo.ProcessPath)
			} else if processInfo.PackageName != "" {
				r.logger.InfoContext(ctx, "found package name: ", processInfo.PackageName)
			} else if processInfo.UserId != -1 {
				if /*needUserName &&*/ true {
					osUser, _ := user.LookupId(F.ToString(processInfo.UserId))
					if osUser != nil {
						processInfo.User = osUser.Username
					}
				}
				if processInfo.User != "" {
					r.logger.InfoContext(ctx, "found user: ", processInfo.User)
				} else {
					r.logger.InfoContext(ctx, "found user id: ", processInfo.UserId)
				}
			}
			metadata.ProcessInfo = processInfo
		}
	}
	if r.fakeIPStore != nil && r.fakeIPStore.Contains(metadata.Destination.Addr) {
		domain, loaded := r.fakeIPStore.Lookup(metadata.Destination.Addr)
		if !loaded {
			fatalErr = E.New("missing fakeip record, try to configure experimental.cache_file")
			return
		}
		metadata.OriginDestination = metadata.Destination
		metadata.Destination = M.Socksaddr{
			Fqdn: domain,
			Port: metadata.Destination.Port,
		}
		metadata.FakeIP = true
		r.logger.DebugContext(ctx, "found fakeip domain: ", domain)
	}
	if r.dnsReverseMapping != nil && metadata.Domain == "" {
		domain, loaded := r.dnsReverseMapping.Query(metadata.Destination.Addr)
		if loaded {
			metadata.Domain = domain
			r.logger.DebugContext(ctx, "found reserve mapped domain: ", metadata.Domain)
		}
	}
	if metadata.Destination.IsIPv4() {
		metadata.IPVersion = 4
	} else if metadata.Destination.IsIPv6() {
		metadata.IPVersion = 6
	}

	//nolint:staticcheck
	if metadata.InboundOptions != common.DefaultValue[option.InboundOptions]() {
		if !preMatch && metadata.InboundOptions.SniffEnabled {
			newBuffers, newErr := r.actionSniff(ctx, metadata, &rule.RuleActionSniff{
				OverrideDestination: metadata.InboundOptions.SniffOverrideDestination,
				Timeout:             time.Duration(metadata.InboundOptions.SniffTimeout),
			}, inputConn, inputPacketConn)
			if newErr != nil {
				fatalErr = newErr
				return
			}
			buffers = append(buffers, newBuffers...)
		}
		if dns.DomainStrategy(metadata.InboundOptions.DomainStrategy) != dns.DomainStrategyAsIS {
			fatalErr = r.actionResolve(ctx, metadata, &rule.RuleActionResolve{
				Strategy: dns.DomainStrategy(metadata.InboundOptions.DomainStrategy),
			})
			if fatalErr != nil {
				return
			}
		}
		if metadata.InboundOptions.UDPDisableDomainUnmapping {
			metadata.UDPDisableDomainUnmapping = true
		}
		metadata.InboundOptions = option.InboundOptions{}
	}

match:
	for ruleIndex < len(r.rules) {
		rules := r.rules
		if ruleIndex != -1 {
			rules = rules[ruleIndex+1:]
		}
		var (
			currentRule      adapter.Rule
			currentRuleIndex int
			matched          bool
		)
		for currentRuleIndex, currentRule = range rules {
			if currentRule.Match(metadata) {
				matched = true
				break
			}
		}
		if !matched {
			break
		}
		if !preMatch {
			r.logger.DebugContext(ctx, "match[", currentRuleIndex, "] ", currentRule, " => ", currentRule.Action())
		} else {
			switch currentRule.Action().Type() {
			case C.RuleActionTypeReject, C.RuleActionTypeResolve:
				r.logger.DebugContext(ctx, "pre-match[", currentRuleIndex, "] ", currentRule, " => ", currentRule.Action())
			}
		}
		switch action := currentRule.Action().(type) {
		case *rule.RuleActionSniff:
			if !preMatch {
				newBuffers, newErr := r.actionSniff(ctx, metadata, action, inputConn, inputPacketConn)
				if newErr != nil {
					fatalErr = newErr
					return
				}
				buffers = append(buffers, newBuffers...)
			} else {
				selectedRule = currentRule
				selectedRuleIndex = currentRuleIndex
				break match
			}
		case *rule.RuleActionResolve:
			fatalErr = r.actionResolve(ctx, metadata, action)
			if fatalErr != nil {
				return
			}
		default:
			selectedRule = currentRule
			selectedRuleIndex = currentRuleIndex
			break match
		}
		ruleIndex = currentRuleIndex
	}
	if !preMatch && metadata.Destination.Addr.IsUnspecified() {
		newBuffers, newErr := r.actionSniff(ctx, metadata, &rule.RuleActionSniff{}, inputConn, inputPacketConn)
		if newErr != nil {
			fatalErr = newErr
			return
		}
		buffers = append(buffers, newBuffers...)
	}
	return
}

func (r *Router) actionSniff(
	ctx context.Context, metadata *adapter.InboundContext, action *rule.RuleActionSniff,
	inputConn net.Conn, inputPacketConn N.PacketConn,
) (buffers []*buf.Buffer, fatalErr error) {
	if sniff.Skip(metadata) {
		return
	} else if inputConn != nil && len(action.StreamSniffers) > 0 {
		buffer := buf.NewPacket()
		err := sniff.PeekStream(
			ctx,
			metadata,
			inputConn,
			buffer,
			action.Timeout,
			action.StreamSniffers...,
		)
		if err == nil {
			//goland:noinspection GoDeprecation
			if action.OverrideDestination && M.IsDomainName(metadata.Domain) {
				metadata.Destination = M.Socksaddr{
					Fqdn: metadata.Domain,
					Port: metadata.Destination.Port,
				}
			}
			if metadata.Domain != "" && metadata.Client != "" {
				r.logger.DebugContext(ctx, "sniffed protocol: ", metadata.Protocol, ", domain: ", metadata.Domain, ", client: ", metadata.Client)
			} else if metadata.Domain != "" {
				r.logger.DebugContext(ctx, "sniffed protocol: ", metadata.Protocol, ", domain: ", metadata.Domain)
			} else {
				r.logger.DebugContext(ctx, "sniffed protocol: ", metadata.Protocol)
			}
		}
		if !buffer.IsEmpty() {
			buffers = append(buffers, buffer)
		} else {
			buffer.Release()
		}
	} else if inputPacketConn != nil && len(action.PacketSniffers) > 0 {
		for {
			var (
				buffer      = buf.NewPacket()
				destination M.Socksaddr
				done        = make(chan struct{})
				err         error
			)
			go func() {
				sniffTimeout := C.ReadPayloadTimeout
				if action.Timeout > 0 {
					sniffTimeout = action.Timeout
				}
				inputPacketConn.SetReadDeadline(time.Now().Add(sniffTimeout))
				destination, err = inputPacketConn.ReadPacket(buffer)
				inputPacketConn.SetReadDeadline(time.Time{})
				close(done)
			}()
			select {
			case <-done:
			case <-ctx.Done():
				inputPacketConn.Close()
				fatalErr = ctx.Err()
				return
			}
			if err != nil {
				buffer.Release()
				if !errors.Is(err, os.ErrDeadlineExceeded) {
					fatalErr = err
					return
				}
			} else {
				// TODO: maybe always override destination
				if metadata.Destination.Addr.IsUnspecified() {
					metadata.Destination = destination
				}
				if len(buffers) > 0 {
					err = sniff.PeekPacket(
						ctx,
						metadata,
						buffer.Bytes(),
						sniff.QUICClientHello,
					)
				} else {
					err = sniff.PeekPacket(
						ctx, metadata,
						buffer.Bytes(),
						action.PacketSniffers...,
					)
				}
				buffers = append(buffers, buffer)
				if E.IsMulti(err, sniff.ErrClientHelloFragmented) && len(buffers) == 0 {
					r.logger.DebugContext(ctx, "attempt to sniff fragmented QUIC client hello")
					continue
				}
				if metadata.Protocol != "" {
					//goland:noinspection GoDeprecation
					if action.OverrideDestination && M.IsDomainName(metadata.Domain) {
						metadata.Destination = M.Socksaddr{
							Fqdn: metadata.Domain,
							Port: metadata.Destination.Port,
						}
					}
					if metadata.Domain != "" && metadata.Client != "" {
						r.logger.DebugContext(ctx, "sniffed packet protocol: ", metadata.Protocol, ", domain: ", metadata.Domain, ", client: ", metadata.Client)
					} else if metadata.Domain != "" {
						r.logger.DebugContext(ctx, "sniffed packet protocol: ", metadata.Protocol, ", domain: ", metadata.Domain)
					} else if metadata.Client != "" {
						r.logger.DebugContext(ctx, "sniffed packet protocol: ", metadata.Protocol, ", client: ", metadata.Client)
					} else {
						r.logger.DebugContext(ctx, "sniffed packet protocol: ", metadata.Protocol)
					}
				}
			}
			break
		}
	}
	return
}

func (r *Router) actionResolve(ctx context.Context, metadata *adapter.InboundContext, action *rule.RuleActionResolve) error {
	if metadata.Destination.IsFqdn() {
		metadata.DNSServer = action.Server
		addresses, err := r.Lookup(adapter.WithContext(ctx, metadata), metadata.Destination.Fqdn, action.Strategy)
		if err != nil {
			return err
		}
		metadata.DestinationAddresses = addresses
		r.dnsLogger.DebugContext(ctx, "resolved [", strings.Join(F.MapToString(metadata.DestinationAddresses), " "), "]")
		if metadata.Destination.IsIPv4() {
			metadata.IPVersion = 4
		} else if metadata.Destination.IsIPv6() {
			metadata.IPVersion = 6
		}
	}
	return nil
}
