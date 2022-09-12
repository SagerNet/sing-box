package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ adapter.Outbound      = (*Selector)(nil)
	_ adapter.OutboundGroup = (*Selector)(nil)
)

type Selector struct {
	myOutboundAdapter
	providerAdapter
	tags       []string
	defaultTag string
	outbounds  map[string]adapter.Outbound
	selected   adapter.Outbound
}

func NewSelector(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.SelectorOutboundOptions, providers []option.ProviderOutboundOptions) (*Selector, error) {
	outbound := &Selector{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeSelector,
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		tags:       options.Outbounds,
		defaultTag: options.Default,
		outbounds:  make(map[string]adapter.Outbound),
	}
	for _, providerOption := range providers {
		provider, err := NewProvider(providerOption.Url, providerOption.Filter, providerOption.Interval, ctx, router, logger)
		if err != nil {
			return nil, err
		}
		outbound.providers = append(outbound.providers, provider)
		go outbound.NewUpdateFunc(&outbound.tags, &outbound.outbounds, router, []func(){
			func() {
				if outbound.selected != nil {
					if _, ok := outbound.outbounds[outbound.selected.Tag()]; ok {
						return
					}
				}
				outbound.InitSelected()
			}})()
	}
	if len(outbound.providers) > 0 {
		outbound.AddCompatibleProxy(&outbound.tags, &outbound.outbounds, router)
	}
	if len(outbound.tags) == 0 {
		return nil, E.New("missing tags")
	}
	return outbound, nil
}

func (s *Selector) Network() []string {
	if s.selected == nil {
		return []string{N.NetworkTCP, N.NetworkUDP}
	}
	return s.selected.Network()
}

func (s *Selector) Start() error {
	for i, tag := range s.tags {
		detour, loaded := s.router.Outbound(tag)
		if !loaded {
			return E.New("outbound ", i, " not found: ", tag)
		}
		s.outbounds[tag] = detour
	}
	return s.InitSelected()
}

func (s *Selector) InitSelected() error {
	if s.tag != "" {
		if clashServer := s.router.ClashServer(); clashServer != nil && clashServer.StoreSelected() {
			selected := clashServer.CacheFile().LoadSelected(s.tag)
			if selected != "" {
				detour, loaded := s.outbounds[selected]
				if loaded {
					s.selected = detour
					return nil
				}
			}
		}
	}

	if s.defaultTag != "" {
		detour, loaded := s.outbounds[s.defaultTag]
		if !loaded {
			return E.New("default outbound not found: ", s.defaultTag)
		}
		s.selected = detour
		return nil
	}

	s.selected = s.outbounds[s.tags[0]]
	return nil
}

func (s *Selector) Now() string {
	return s.selected.Tag()
}

func (s *Selector) All() []string {
	return s.tags
}

func (s *Selector) SelectOutbound(tag string) bool {
	detour, loaded := s.outbounds[tag]
	if !loaded {
		return false
	}
	s.selected = detour
	if s.tag != "" {
		if clashServer := s.router.ClashServer(); clashServer != nil && clashServer.StoreSelected() {
			err := clashServer.CacheFile().StoreSelected(s.tag, tag)
			if err != nil {
				s.logger.Error("store selected: ", err)
			}
		}
	}
	return true
}

func (s *Selector) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return s.selected.DialContext(ctx, network, destination)
}

func (s *Selector) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return s.selected.ListenPacket(ctx, destination)
}

func (s *Selector) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return s.selected.NewConnection(ctx, conn, metadata)
}

func (s *Selector) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return s.selected.NewPacketConnection(ctx, conn, metadata)
}

func RealTag(detour adapter.Outbound) string {
	if group, isGroup := detour.(adapter.OutboundGroup); isGroup {
		return group.Now()
	}
	return detour.Tag()
}
