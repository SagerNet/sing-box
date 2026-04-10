//go:build darwin

package local

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport/hosts"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
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
	fallback      bool
	dhcpTransport dhcpTransport
}

type dhcpTransport interface {
	adapter.DNSTransport
	Fetch() []M.Socksaddr
	Exchange0(ctx context.Context, message *mDNS.Msg, servers []M.Socksaddr) (*mDNS.Msg, error)
}

func NewTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.LocalDNSServerOptions) (adapter.DNSTransport, error) {
	transportDialer, err := dns.NewLocalDialer(ctx, options)
	if err != nil {
		return nil, err
	}
	return &Transport{
		TransportAdapter: dns.NewTransportAdapterWithLocalOptions(C.DNSTypeLocal, tag, options),
		ctx:              ctx,
		logger:           logger,
		hosts:            hosts.NewFile(hosts.DefaultPath),
		dialer:           transportDialer,
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
	if t.fallback {
		t.dhcpTransport = newDHCPTransport(t.TransportAdapter, log.ContextWithOverrideLevel(t.ctx, log.LevelDebug), t.dialer, t.logger)
		if t.dhcpTransport != nil {
			err := t.dhcpTransport.Start(stage)
			if err != nil {
				return err
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

func (t *Transport) Reset() {
	if t.dhcpTransport != nil {
		t.dhcpTransport.Reset()
	}
}
