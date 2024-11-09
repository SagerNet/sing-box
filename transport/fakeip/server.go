package fakeip

import (
	"context"
	"net/netip"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-dns"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

var (
	_ dns.Transport           = (*Transport)(nil)
	_ adapter.FakeIPTransport = (*Transport)(nil)
)

func init() {
	dns.RegisterTransport([]string{"fakeip"}, func(options dns.TransportOptions) (dns.Transport, error) {
		return NewTransport(options)
	})
}

type Transport struct {
	name   string
	router adapter.Router
	store  adapter.FakeIPStore
	logger logger.ContextLogger
}

func NewTransport(options dns.TransportOptions) (*Transport, error) {
	router := service.FromContext[adapter.Router](options.Context)
	if router == nil {
		return nil, E.New("missing router in context")
	}
	return &Transport{
		name:   options.Name,
		router: router,
		logger: options.Logger,
	}, nil
}

func (s *Transport) Name() string {
	return s.name
}

func (s *Transport) Start() error {
	s.store = s.router.FakeIPStore()
	if s.store == nil {
		return E.New("fakeip not enabled")
	}
	return nil
}

func (s *Transport) Reset() {
}

func (s *Transport) Close() error {
	return nil
}

func (s *Transport) Raw() bool {
	return false
}

func (s *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	return nil, os.ErrInvalid
}

func (s *Transport) Lookup(ctx context.Context, domain string, strategy dns.DomainStrategy) ([]netip.Addr, error) {
	var addresses []netip.Addr
	if strategy != dns.DomainStrategyUseIPv6 {
		inet4Address, err := s.store.Create(domain, false)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, inet4Address)
	}
	if strategy != dns.DomainStrategyUseIPv4 {
		inet6Address, err := s.store.Create(domain, true)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, inet6Address)
	}
	return addresses, nil
}

func (s *Transport) Store() adapter.FakeIPStore {
	return s.store
}
