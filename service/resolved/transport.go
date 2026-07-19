//go:build linux

package resolved

import (
	"context"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.ResolvedDNSServerOptions](registry, C.TypeResolved, NewTransport)
}

var (
	_ adapter.DNSTransport                    = (*Transport)(nil)
	_ adapter.DNSTransportWithPreferredDomain = (*Transport)(nil)
)

type Transport struct {
	dns.TransportAdapter
	ctx                    context.Context
	logger                 logger.ContextLogger
	serviceTag             string
	acceptDefaultResolvers bool
	ndots                  int
	timeout                time.Duration
	attempts               int
	rotate                 bool
	service                *Service
	linkAccess             sync.RWMutex
	linkServers            map[*TransportLink]*LinkServers
}

type LinkServers struct {
	Link         *TransportLink
	Servers      []adapter.DNSTransport
	serverOffset uint32
}

func (c *LinkServers) ServerOffset(rotate bool) uint32 {
	if rotate {
		return atomic.AddUint32(&c.serverOffset, 1) - 1
	}
	return 0
}

func NewTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.ResolvedDNSServerOptions) (adapter.DNSTransport, error) {
	return &Transport{
		TransportAdapter:       dns.NewTransportAdapter(C.DNSTypeDHCP, tag, nil),
		ctx:                    ctx,
		logger:                 logger,
		serviceTag:             options.Service,
		acceptDefaultResolvers: options.AcceptDefaultResolvers,
		// ndots:                  options.NDots,
		// timeout:                time.Duration(options.Timeout),
		// attempts:               options.Attempts,
		// rotate:                 options.Rotate,
		ndots:       1,
		timeout:     5 * time.Second,
		attempts:    2,
		linkServers: make(map[*TransportLink]*LinkServers),
	}, nil
}

func (t *Transport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateInitialize {
		return nil
	}
	serviceManager := service.FromContext[adapter.ServiceManager](t.ctx)
	service, loaded := serviceManager.Get(t.serviceTag)
	if !loaded {
		return E.New("service not found: ", t.serviceTag)
	}
	resolvedInbound, isResolved := service.(*Service)
	if !isResolved {
		return E.New("service is not resolved: ", t.serviceTag)
	}
	resolvedInbound.updateCallback = t.updateTransports
	resolvedInbound.deleteCallback = t.deleteTransport
	t.service = resolvedInbound
	return nil
}

func (t *Transport) Close() error {
	t.linkAccess.RLock()
	defer t.linkAccess.RUnlock()
	for _, servers := range t.linkServers {
		for _, server := range servers.Servers {
			server.Close()
		}
	}
	return nil
}

func (t *Transport) Reset() {
	t.linkAccess.RLock()
	defer t.linkAccess.RUnlock()
	for _, servers := range t.linkServers {
		for _, server := range servers.Servers {
			server.Reset()
		}
	}
}

func (t *Transport) updateTransports(link *TransportLink) error {
	t.linkAccess.Lock()
	defer t.linkAccess.Unlock()
	if servers, loaded := t.linkServers[link]; loaded {
		for _, server := range servers.Servers {
			server.Close()
		}
	}
	serverDialer := common.Must1(dialer.NewDefault(t.ctx, option.DialerOptions{
		BindInterface:      link.iif.Name,
		UDPFragmentDefault: true,
	}))
	var transports []adapter.DNSTransport
	for _, address := range link.address {
		serverAddr, ok := netip.AddrFromSlice(address.Address)
		if !ok {
			return os.ErrInvalid
		}
		if link.dnsOverTLS {
			tlsConfig := common.Must1(tls.NewClient(t.ctx, t.logger, serverAddr.String(), option.OutboundTLSOptions{
				Enabled:    true,
				ServerName: serverAddr.String(),
			}))
			transports = append(transports, transport.NewTLSRaw(t.logger, t.TransportAdapter, serverDialer, M.SocksaddrFrom(serverAddr, 53), tlsConfig))

		} else {
			transports = append(transports, transport.NewUDPRaw(t.logger, t.TransportAdapter, serverDialer, M.SocksaddrFrom(serverAddr, 53)))
		}
	}
	for _, address := range link.addressEx {
		serverAddr, ok := netip.AddrFromSlice(address.Address)
		if !ok {
			return os.ErrInvalid
		}
		if link.dnsOverTLS {
			var serverName string
			if address.Name != "" {
				serverName = address.Name
			} else {
				serverName = serverAddr.String()
			}
			tlsConfig := common.Must1(tls.NewClient(t.ctx, t.logger, serverAddr.String(), option.OutboundTLSOptions{
				Enabled:    true,
				ServerName: serverName,
			}))
			transports = append(transports, transport.NewTLSRaw(t.logger, t.TransportAdapter, serverDialer, M.SocksaddrFrom(serverAddr, address.Port), tlsConfig))

		} else {
			transports = append(transports, transport.NewUDPRaw(t.logger, t.TransportAdapter, serverDialer, M.SocksaddrFrom(serverAddr, address.Port)))
		}
	}
	t.linkServers[link] = &LinkServers{
		Link:    link,
		Servers: transports,
	}
	return nil
}

func (t *Transport) deleteTransport(link *TransportLink) {
	t.linkAccess.Lock()
	defer t.linkAccess.Unlock()
	servers, loaded := t.linkServers[link]
	if !loaded {
		return
	}
	for _, server := range servers.Servers {
		server.Close()
	}
	delete(t.linkServers, link)
}

func (t *Transport) PreferredDomain(domain string) bool {
	t.service.linkAccess.RLock()
	defer t.service.linkAccess.RUnlock()
	for _, link := range t.service.links {
		for _, linkDomain := range link.domain {
			if linkDomain.Domain == "." {
				continue
			}
			if mDNS.IsSubDomain(linkDomain.Domain, domain) {
				return true
			}
		}
	}
	return false
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	done := make(chan struct{})
	var (
		response *mDNS.Msg
		err      error
	)
	t.ExchangeAsync(ctx, message, func(callbackResponse *mDNS.Msg, callbackErr error) {
		response = callbackResponse
		err = callbackErr
		close(done)
	})
	<-done
	return response, err
}

func (t *Transport) ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	question := message.Question[0]
	var selectedLink *TransportLink
	t.service.linkAccess.RLock()
	for _, link := range t.service.links {
		for _, domain := range link.domain {
			if domain.Domain == "." && domain.RoutingOnly && !t.acceptDefaultResolvers {
				continue
			}
			if mDNS.IsSubDomain(domain.Domain, question.Name) {
				selectedLink = link
			}
		}
	}
	if selectedLink == nil && t.acceptDefaultResolvers {
		for l := len(t.service.defaultRouteSequence); l > 0; l-- {
			selectedLink = t.service.links[t.service.defaultRouteSequence[l-1]]
			if len(selectedLink.address) > 0 || len(selectedLink.addressEx) > 0 {
				break
			}
		}
	}
	t.service.linkAccess.RUnlock()
	if selectedLink == nil {
		callback(dns.FixedResponseStatus(message, mDNS.RcodeNameError), nil)
		return
	}
	t.linkAccess.RLock()
	servers := t.linkServers[selectedLink]
	t.linkAccess.RUnlock()
	if servers == nil || len(servers.Servers) == 0 {
		callback(dns.FixedResponseStatus(message, mDNS.RcodeNameError), nil)
		return
	}
	names := servers.Link.nameList(t.ndots, question.Name)
	if len(names) == 0 {
		callback(nil, E.New("invalid domain: ", question.Name))
		return
	}
	nameExchangers := make([]transport.AsyncExchanger, 0, len(names))
	for _, fqdn := range names {
		nameExchangers = append(nameExchangers, t.newNameExchanger(servers, message, fqdn))
	}
	if question.Qtype == mDNS.TypeA || question.Qtype == mDNS.TypeAAAA {
		transport.ExchangeRace(ctx, nameExchangers, callback)
	} else {
		transport.ExchangeSequential(ctx, nameExchangers, nil, callback)
	}
}

func (t *Transport) newNameExchanger(servers *LinkServers, message *mDNS.Msg, fqdn string) transport.AsyncExchanger {
	serverOffset := servers.ServerOffset(t.rotate)
	serverCount := uint32(len(servers.Servers))
	attemptExchangers := make([]transport.AsyncExchanger, 0, t.attempts*int(serverCount))
	for i := 0; i < t.attempts; i++ {
		for j := range serverCount {
			server := servers.Servers[(serverOffset+j)%serverCount]
			attemptExchangers = append(attemptExchangers, func(ctx context.Context, callback func(response *mDNS.Msg, err error)) {
				question := message.Question[0]
				question.Name = fqdn
				exchangeMessage := *message
				exchangeMessage.Question = []mDNS.Question{question}
				exchangeCtx, cancel := context.WithTimeout(ctx, t.timeout)
				server.ExchangeAsync(exchangeCtx, &exchangeMessage, func(response *mDNS.Msg, err error) {
					cancel()
					callback(response, err)
				})
			})
		}
	}
	return func(ctx context.Context, callback func(response *mDNS.Msg, err error)) {
		transport.ExchangeSequential(ctx, attemptExchangers, nil, func(response *mDNS.Msg, err error) {
			if err != nil {
				err = E.Cause(err, fqdn)
			}
			callback(response, err)
		})
	}
}
