package split

import (
	"context"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"

	"github.com/godbus/dbus/v5"
	mDNS "github.com/miekg/dns"
)

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.SplitDNSServerOptions](registry, C.DNSTypeSplitDNS, NewTransport)
}

var _ adapter.DNSTransport = (*Transport)(nil)

type Transport struct {
	dns.TransportAdapter
	ctx                    context.Context
	network                adapter.NetworkManager
	dnsRouter              adapter.DNSRouter
	logger                 logger.ContextLogger
	acceptDefaultResolvers bool
	linkAccess             sync.Mutex
	links                  map[uint32]*TransportLink
}

type TransportLink struct {
	iif          *control.Interface
	nameservers  []adapter.DNSTransport
	domains      []resolve1LinkDomain
	defaultRoute bool
	dnsOverTLS   bool
}

func NewTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.SplitDNSServerOptions) (adapter.DNSTransport, error) {
	if !C.IsLinux {
		return nil, E.New("split DNS server is only supported on Linux")
	}
	return &Transport{
		TransportAdapter:       dns.NewTransportAdapter(C.DNSTypeDHCP, tag, nil),
		ctx:                    ctx,
		logger:                 logger,
		acceptDefaultResolvers: options.AcceptDefaultResolvers,
		network:                service.FromContext[adapter.NetworkManager](ctx),
		dnsRouter:              service.FromContext[adapter.DNSRouter](ctx),
		links:                  make(map[uint32]*TransportLink),
	}, nil
}

func (t *Transport) Start(stage adapter.StartStage) error {
	switch stage {
	case adapter.StartStateInitialize:
		dnsTransportManager := service.FromContext[adapter.DNSTransportManager](t.ctx)
		for _, transport := range dnsTransportManager.Transports() {
			if transport.Type() == C.DNSTypeSplitDNS && transport != t {
				return E.New("multiple split DNS server are not supported")
			}
		}
	case adapter.StartStateStart:
		systemBus, err := dbus.SystemBus()
		if err != nil {
			return err
		}
		reply, err := systemBus.RequestName("org.freedesktop.resolve1", dbus.NameFlagDoNotQueue)
		if err != nil {
			return err
		}
		switch reply {
		case dbus.RequestNameReplyPrimaryOwner:
		case dbus.RequestNameReplyExists:
			return E.New("D-Bus object already exists, maybe real resolved is running")
		default:
			return E.New("unknown request name reply: ", reply)
		}
		err = systemBus.Export((*resolve1Manager)(t), "/org/freedesktop/resolve1", "org.freedesktop.resolve1.Manager")
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Transport) Close() error {
	return nil
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	question := message.Question[0]
	var selectedLink *TransportLink
	for _, link := range t.links {
		for _, domain := range link.domains {
			if domain.RoutingOnly && !t.acceptDefaultResolvers {
				continue
			}
			if strings.HasSuffix(question.Name, domain.Domain) {
				selectedLink = link
			}
		}
	}
	if selectedLink == nil && t.acceptDefaultResolvers {
		for _, link := range t.links {
			if link.defaultRoute {
				selectedLink = link
			}
		}
	}
	if selectedLink == nil {
		return nil, dns.RcodeNameError
	}
	if question.Qtype == mDNS.TypeA || question.Qtype == mDNS.TypeAAAA {
		return t.exchangeParallel(ctx, selectedLink.nameservers, message)
	} else {
		return t.exchangeSingleRequest(ctx, selectedLink.nameservers, message)
	}
}

func (t *Transport) exchangeSingleRequest(ctx context.Context, transports []adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
	var errors []error
	for _, transport := range transports {
		response, err := transport.Exchange(ctx, message)
		if err == nil {
			addresses, _ := dns.MessageToAddresses(response)
			if len(addresses) == 0 {
				err = E.New("empty result")
			}
		}
		if err != nil {
			errors = append(errors, err)
		} else {
			return response, nil
		}
	}
	return nil, E.Errors(errors...)
}

func (t *Transport) exchangeParallel(ctx context.Context, transports []adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
	returned := make(chan struct{})
	defer close(returned)
	type queryResult struct {
		response *mDNS.Msg
		err      error
	}
	results := make(chan queryResult)
	startRacer := func(ctx context.Context, transport adapter.DNSTransport) {
		response, err := transport.Exchange(ctx, message)
		if err == nil {
			addresses, _ := dns.MessageToAddresses(response)
			if len(addresses) == 0 {
				err = E.New("empty result")
			}
		}
		select {
		case results <- queryResult{response, err}:
		case <-returned:
		}
	}
	queryCtx, queryCancel := context.WithCancel(ctx)
	defer queryCancel()
	var nameCount int
	for _, fqdn := range transports {
		nameCount++
		go startRacer(queryCtx, fqdn)
	}
	var errors []error
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-results:
			if result.err == nil {
				return result.response, nil
			}
			errors = append(errors, result.err)
			if len(errors) == nameCount {
				return nil, E.Errors(errors...)
			}
		}
	}
}
