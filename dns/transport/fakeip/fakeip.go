package fakeip

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	mDNS "github.com/miekg/dns"
)

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.FakeIPDNSServerOptions](registry, C.DNSTypeFakeIP, NewTransport)
}

var _ adapter.FakeIPTransport = (*Transport)(nil)

type Transport struct {
	dns.TransportAdapter
	logger logger.ContextLogger
	store  adapter.FakeIPStore
}

func NewTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.FakeIPDNSServerOptions) (adapter.DNSTransport, error) {
	store := NewStore(ctx, logger, options.Inet4Range.Build(netip.Prefix{}), options.Inet6Range.Build(netip.Prefix{}))
	return &Transport{
		TransportAdapter: dns.NewTransportAdapter(C.DNSTypeFakeIP, tag, nil),
		logger:           logger,
		store:            store,
	}, nil
}

func (t *Transport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return t.store.Start()
}

func (t *Transport) Close() error {
	return t.store.Close()
}

func (t *Transport) Reset() {
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	question := message.Question[0]
	if question.Qtype != mDNS.TypeA && question.Qtype != mDNS.TypeAAAA {
		return nil, E.New("only IP queries are supported by fakeip")
	}
	address, err := t.store.Create(dns.FqdnToDomain(question.Name), question.Qtype == mDNS.TypeAAAA)
	if err != nil {
		return nil, err
	}
	return dns.FixedResponse(message.Id, question, []netip.Addr{address}, C.DefaultDNSTTL), nil
}

func (t *Transport) Store() adapter.FakeIPStore {
	return t.store
}
