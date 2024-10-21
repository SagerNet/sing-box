package adapter

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing-box/common/process"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
)

type Inbound interface {
	Service
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

	// cache

	InboundDetour     string
	LastInbound       string
	OriginDestination M.Socksaddr
	// Deprecated
	InboundOptions            option.InboundOptions
	UDPDisableDomainUnmapping bool
	DestinationAddresses      []netip.Addr
	SourceGeoIPCode           string
	GeoIPCode                 string
	ProcessInfo               *process.Info
	QueryType                 uint16
	FakeIP                    bool

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
	c.SourceAddressMatch = false
	c.SourcePortMatch = false
	c.DestinationAddressMatch = false
	c.DestinationPortMatch = false
	c.DidMatch = false
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
		var newMetadata InboundContext
		newMetadata = *metadata
		return WithContext(ctx, &newMetadata)
	}
	return ctx
}
