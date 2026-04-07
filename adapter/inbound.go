package adapter

import (
	"context"
	"net"
	"net/netip"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/miekg/dns"
)

type Inbound interface {
	Lifecycle
	Type() string
	Tag() string
}

type TCPInjectableInbound interface {
	Inbound
	ConnectionHandlerEx
}

type UDPInjectableInbound interface {
	Inbound
	PacketConnectionHandlerEx
}

type InboundRegistry interface {
	option.InboundOptionsRegistry
	Create(ctx context.Context, router Router, logger log.ContextLogger, tag string, inboundType string, options any) (Inbound, error)
}

type InboundManager interface {
	Lifecycle
	Inbounds() []Inbound
	Get(tag string) (Inbound, bool)
	Remove(tag string) error
	Create(ctx context.Context, router Router, logger log.ContextLogger, tag string, inboundType string, options any) error
}

type InboundContext struct {
	Inbound     string
	InboundType string
	IPVersion   uint8
	Network     string
	Source      M.Socksaddr
	Destination M.Socksaddr
	User        string
	Outbound    string

	// sniffer

	Protocol     string
	Domain       string
	Client       string
	SniffContext any
	SnifferNames []string
	SniffError   error

	// cache

	// Deprecated: implement in rule action
	InboundDetour             string
	LastInbound               string
	OriginDestination         M.Socksaddr
	RouteOriginalDestination  M.Socksaddr
	UDPDisableDomainUnmapping bool
	UDPConnect                bool
	UDPTimeout                time.Duration
	TLSFragment               bool
	TLSFragmentFallbackDelay  time.Duration
	TLSRecordFragment         bool

	NetworkStrategy     *C.NetworkStrategy
	NetworkType         []C.InterfaceType
	FallbackNetworkType []C.InterfaceType
	FallbackDelay       time.Duration

	DestinationAddresses                []netip.Addr
	DNSResponse                         *dns.Msg
	DestinationAddressMatchFromResponse bool
	SourceGeoIPCode                     string
	GeoIPCode                           string
	ProcessInfo                         *ConnectionOwner
	SourceMACAddress                    net.HardwareAddr
	SourceHostname                      string
	QueryType                           uint16
	FakeIP                              bool

	// rule cache

	IPCIDRMatchSource bool
	IPCIDRAcceptEmpty bool

	SourceAddressMatch           bool
	SourcePortMatch              bool
	DestinationAddressMatch      bool
	DestinationPortMatch         bool
	DidMatch                     bool
	IgnoreDestinationIPCIDRMatch bool
}

func (c *InboundContext) ResetRuleCache() {
	c.IPCIDRMatchSource = false
	c.IPCIDRAcceptEmpty = false
	c.ResetRuleMatchCache()
}

func (c *InboundContext) ResetRuleMatchCache() {
	c.SourceAddressMatch = false
	c.SourcePortMatch = false
	c.DestinationAddressMatch = false
	c.DestinationPortMatch = false
	c.DidMatch = false
}

func (c *InboundContext) DNSResponseAddressesForMatch() []netip.Addr {
	return DNSResponseAddresses(c.DNSResponse)
}

func DNSResponseAddresses(response *dns.Msg) []netip.Addr {
	if response == nil || response.Rcode != dns.RcodeSuccess {
		return nil
	}
	addresses := make([]netip.Addr, 0, len(response.Answer))
	for _, rawRecord := range response.Answer {
		switch record := rawRecord.(type) {
		case *dns.A:
			addr := M.AddrFromIP(record.A)
			if addr.IsValid() {
				addresses = append(addresses, addr)
			}
		case *dns.AAAA:
			addr := M.AddrFromIP(record.AAAA)
			if addr.IsValid() {
				addresses = append(addresses, addr)
			}
		case *dns.HTTPS:
			for _, value := range record.SVCB.Value {
				switch hint := value.(type) {
				case *dns.SVCBIPv4Hint:
					for _, ip := range hint.Hint {
						addr := M.AddrFromIP(ip).Unmap()
						if addr.IsValid() {
							addresses = append(addresses, addr)
						}
					}
				case *dns.SVCBIPv6Hint:
					for _, ip := range hint.Hint {
						addr := M.AddrFromIP(ip)
						if addr.IsValid() {
							addresses = append(addresses, addr)
						}
					}
				}
			}
		}
	}
	return addresses
}

type inboundContextKey struct{}

func WithContext(ctx context.Context, inboundContext *InboundContext) context.Context {
	return context.WithValue(ctx, (*inboundContextKey)(nil), inboundContext)
}

func ContextFrom(ctx context.Context) *InboundContext {
	metadata := ctx.Value((*inboundContextKey)(nil))
	if metadata == nil {
		return nil
	}
	return metadata.(*InboundContext)
}

func ExtendContext(ctx context.Context) (context.Context, *InboundContext) {
	var newMetadata InboundContext
	if metadata := ContextFrom(ctx); metadata != nil {
		newMetadata = *metadata
	}
	return WithContext(ctx, &newMetadata), &newMetadata
}

func OverrideContext(ctx context.Context) context.Context {
	if metadata := ContextFrom(ctx); metadata != nil {
		newMetadata := *metadata
		return WithContext(ctx, &newMetadata)
	}
	return ctx
}
