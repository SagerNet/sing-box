package local

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport/local/systemconfig"
	"github.com/sagernet/sing-box/dns/transport/mdns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"

	mDNS "github.com/miekg/dns"
)

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.LocalDNSServerOptions](registry, C.DNSTypeLocal, NewTransport)
}

var (
	_ adapter.DNSTransport                    = (*Transport)(nil)
	_ adapter.DNSTransportWithPreferredDomain = (*Transport)(nil)
	_ adapter.DNSTransportWithEnvironment     = (*Transport)(nil)
)

type Transport struct {
	dns.TransportAdapter
	ctx               context.Context
	logger            logger.ContextLogger
	preferredResolver *PreferredDomainResolver
	dialer            N.Dialer
	preferGo          bool
	resolved          ResolvedResolver
	mdnsTransport     adapter.DNSTransport
	configSource      *systemconfig.Source
	system            systemResolver
	serverSet         atomic.Pointer[localServerSet]
	serverSetAccess   sync.Mutex
}

func NewTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.LocalDNSServerOptions) (adapter.DNSTransport, error) {
	transportDialer, err := dns.NewLocalDialer(ctx, options)
	if err != nil {
		return nil, err
	}
	preferredResolver, err := NewPreferredDomainResolver(ctx, logger, options)
	if err != nil {
		return nil, err
	}
	return &Transport{
		TransportAdapter:  dns.NewTransportAdapterWithLocalOptions(C.DNSTypeLocal, tag, options),
		ctx:               ctx,
		logger:            logger,
		preferredResolver: preferredResolver,
		dialer:            transportDialer,
		preferGo:          options.PreferGo,
		configSource:      systemconfig.NewSource(ctx),
	}, nil
}

func (t *Transport) Start(stage adapter.StartStage) error {
	t.preferredResolver.Start(stage)
	switch stage {
	case adapter.StartStateInitialize:
		if !t.preferGo && isSystemdResolvedManaged() {
			resolvedResolver, err := NewResolvedResolver(t.ctx, t.logger)
			if err == nil {
				err = resolvedResolver.Start()
				if err == nil {
					t.resolved = resolvedResolver
				} else {
					t.logger.Warn(E.Cause(err, "initialize resolved resolver"))
				}
			}
		}
	case adapter.StartStateStart:
		if !C.IsDarwin {
			t.mdnsTransport = mdns.NewRawTransport(t.TransportAdapter, t.ctx, t.logger)
		}
		fallthrough
	default:
		if t.mdnsTransport != nil {
			err := t.mdnsTransport.Start(stage)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Transport) Close() error {
	serverSet := t.serverSet.Swap(nil)
	if serverSet != nil {
		serverSet.Close()
	}
	t.system.close()
	return common.Close(t.resolved, t.mdnsTransport, t.configSource)
}

func (t *Transport) Reset() {
	serverSet := t.serverSet.Load()
	if serverSet != nil {
		for _, serverTransport := range serverSet.transports {
			serverTransport.Reset()
		}
	}
	t.system.reset()
	t.configSource.Reset()
	if t.resolved != nil {
		t.resolved.Reset()
	}
	if t.mdnsTransport != nil {
		t.mdnsTransport.Reset()
	}
}

func (t *Transport) PreferredDomain(domain string) bool {
	return t.preferredResolver.PreferredDomain(domain)
}

func (t *Transport) Environment() []string {
	if t.resolved != nil {
		return t.resolved.Environment()
	}
	return t.configSource.Configuration().Signature()
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
	response := t.preferredResolver.Lookup(message)
	if response != nil {
		callback(response, nil)
		return
	}
	if mdns.IsLocalDomain(question.Name) {
		if C.IsDarwin {
			t.systemExchangeAsync(ctx, message, callback)
			return
		}
		t.mdnsTransport.ExchangeAsync(ctx, message, callback)
		return
	}
	if t.resolved != nil {
		t.resolved.ExchangeAsync(ctx, message, callback)
		return
	}
	t.exchangeAsync(ctx, message, question.Name, callback)
}
