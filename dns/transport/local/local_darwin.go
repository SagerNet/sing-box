//go:build darwin

package local

import (
	"context"
	"errors"
	"net"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport/hosts"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.LocalDNSServerOptions](registry, C.DNSTypeLocal, NewTransport)
}

var _ adapter.DNSTransport = (*Transport)(nil)

type Transport struct {
	dns.TransportAdapter
	ctx           context.Context
	logger        logger.ContextLogger
	hosts         *hosts.File
	dialer        N.Dialer
	preferGo      bool
	fallback      bool
	dhcpTransport dhcpTransport
	resolver      net.Resolver
}

type dhcpTransport interface {
	adapter.DNSTransport
	Fetch() ([]M.Socksaddr, error)
	Exchange0(ctx context.Context, message *mDNS.Msg, servers []M.Socksaddr) (*mDNS.Msg, error)
}

func NewTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.LocalDNSServerOptions) (adapter.DNSTransport, error) {
	transportDialer, err := dns.NewLocalDialer(ctx, options)
	if err != nil {
		return nil, err
	}
	transportAdapter := dns.NewTransportAdapterWithLocalOptions(C.DNSTypeLocal, tag, options)
	return &Transport{
		TransportAdapter: transportAdapter,
		ctx:              ctx,
		logger:           logger,
		hosts:            hosts.NewFile(hosts.DefaultPath),
		dialer:           transportDialer,
		preferGo:         options.PreferGo,
	}, nil
}

func (t *Transport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	inboundManager := service.FromContext[adapter.InboundManager](t.ctx)
	for _, inbound := range inboundManager.Inbounds() {
		if inbound.Type() == C.TypeTun {
			t.fallback = true
			break
		}
	}
	if !C.IsIos {
		if t.fallback {
			t.dhcpTransport = newDHCPTransport(t.TransportAdapter, log.ContextWithOverrideLevel(t.ctx, log.LevelDebug), t.dialer, t.logger)
			if t.dhcpTransport != nil {
				err := t.dhcpTransport.Start(stage)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (t *Transport) Close() error {
	return common.Close(
		t.dhcpTransport,
	)
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	question := message.Question[0]
	if question.Qtype == mDNS.TypeA || question.Qtype == mDNS.TypeAAAA {
		addresses := t.hosts.Lookup(dns.FqdnToDomain(question.Name))
		if len(addresses) > 0 {
			return dns.FixedResponse(message.Id, question, addresses, C.DefaultDNSTTL), nil
		}
	}
	if !t.fallback {
		return t.exchange(ctx, message, question.Name)
	}
	if !C.IsIos {
		if t.dhcpTransport != nil {
			dhcpTransports, _ := t.dhcpTransport.Fetch()
			if len(dhcpTransports) > 0 {
				return t.dhcpTransport.Exchange0(ctx, message, dhcpTransports)
			}
		}
	}
	if t.preferGo {
		// Assuming the user knows what they are doing, we still execute the query which will fail.
		return t.exchange(ctx, message, question.Name)
	}
	if question.Qtype == mDNS.TypeA || question.Qtype == mDNS.TypeAAAA {
		var network string
		if question.Qtype == mDNS.TypeA {
			network = "ip4"
		} else {
			network = "ip6"
		}
		addresses, err := t.resolver.LookupNetIP(ctx, network, question.Name)
		if err != nil {
			var dnsError *net.DNSError
			if errors.As(err, &dnsError) && dnsError.IsNotFound {
				return nil, dns.RcodeRefused
			}
			return nil, err
		}
		return dns.FixedResponse(message.Id, question, addresses, C.DefaultDNSTTL), nil
	}
	if C.IsIos {
		return nil, E.New("only A and AAAA queries are supported on iOS and tvOS when using NetworkExtension.")
	} else {
		return nil, E.New("only A and AAAA queries are supported on macOS when using NetworkExtension and DHCP unavailable.")
	}
}
