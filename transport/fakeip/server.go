package fakeip

import (
	"context"
	"net/netip"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-dns"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"

	mDNS "github.com/miekg/dns"
)

var _ dns.Transport = (*Server)(nil)

func init() {
	dns.RegisterTransport([]string{"fakeip"}, NewTransport)
}

type Server struct {
	router adapter.Router
	store  adapter.FakeIPStore
	logger logger.ContextLogger
}

func NewTransport(ctx context.Context, logger logger.ContextLogger, dialer N.Dialer, link string) (dns.Transport, error) {
	router := adapter.RouterFromContext(ctx)
	if router == nil {
		return nil, E.New("missing router in context")
	}
	return &Server{
		router: router,
		logger: logger,
	}, nil
}

func (s *Server) Start() error {
	s.store = s.router.FakeIPStore()
	if s.store == nil {
		return E.New("fakeip not enabled")
	}
	return nil
}

func (s *Server) Close() error {
	return nil
}

func (s *Server) Raw() bool {
	return false
}

func (s *Server) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	return nil, os.ErrInvalid
}

func (s *Server) Lookup(ctx context.Context, domain string, strategy dns.DomainStrategy) ([]netip.Addr, error) {
	var addresses []netip.Addr
	if strategy != dns.DomainStrategyUseIPv6 {
		inet4Address, err := s.store.Create(domain, dns.DomainStrategyUseIPv4)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, inet4Address)
	}
	if strategy != dns.DomainStrategyUseIPv4 {
		inet6Address, err := s.store.Create(domain, dns.DomainStrategyUseIPv6)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, inet6Address)
	}
	return addresses, nil
}
