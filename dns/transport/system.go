package transport

import (
	"context"
	"errors"
	"net"

	mDNS "github.com/miekg/dns"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
)

func RegisterSystem(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.SystemDNSServerOptions](registry, C.DNSTypeSystem, NewTransport)
}

var _ adapter.DNSTransport = (*Transport)(nil)

type Transport struct {
	dns.TransportAdapter
	ctx      context.Context
	logger   logger.ContextLogger
	resolver net.Resolver
}

func NewTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.SystemDNSServerOptions) (adapter.DNSTransport, error) {
	return &Transport{
		TransportAdapter: dns.NewTransportAdapter(C.DNSTypeSystem, tag, []string{}),
		ctx:              ctx,
		logger:           logger,
	}, nil
}

func (t *Transport) Start(_ adapter.StartStage) error {
	return nil
}

func (t *Transport) Close() error {
	return nil
}

func (t *Transport) Reset() {
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	question := message.Question[0]
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
	return nil, dns.RcodeRefused
}
