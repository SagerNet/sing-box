package adapter

import (
	"context"
	"net/http"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/common/geoip"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"

	mdns "github.com/miekg/dns"
	"go4.org/netipx"
)

type Router interface {
	Service
	PreStarter
	PostStarter
	Cleanup() error

	Outbounds() []Outbound
	Outbound(tag string) (Outbound, bool)
	DefaultOutbound(network string) (Outbound, error)

	FakeIPStore() FakeIPStore

	ConnectionRouter

	GeoIPReader() *geoip.Reader
	LoadGeosite(code string) (Rule, error)

	RuleSets() []RuleSet
	RuleSet(tag string) (RuleSet, bool)

	NeedWIFIState() bool

	Exchange(ctx context.Context, message *mdns.Msg) (*mdns.Msg, error)
	Lookup(ctx context.Context, domain string, strategy dns.DomainStrategy) ([]netip.Addr, error)
	LookupDefault(ctx context.Context, domain string) ([]netip.Addr, error)
	ClearDNSCache()

	InterfaceFinder() control.InterfaceFinder
	UpdateInterfaces() error
	DefaultInterface() string
	AutoDetectInterface() bool
	AutoDetectInterfaceFunc() control.Func
	DefaultMark() uint32
	RegisterAutoRedirectOutputMark(mark uint32) error
	AutoRedirectOutputMark() uint32
	NetworkMonitor() tun.NetworkUpdateMonitor
	InterfaceMonitor() tun.DefaultInterfaceMonitor
	PackageManager() tun.PackageManager
	WIFIState() WIFIState
	Rules() []Rule

	ClashServer() ClashServer
	SetClashServer(server ClashServer)

	V2RayServer() V2RayServer
	SetV2RayServer(server V2RayServer)

	ResetNetwork() error
}

func ContextWithRouter(ctx context.Context, router Router) context.Context {
	return service.ContextWith(ctx, router)
}

func RouterFromContext(ctx context.Context) Router {
	return service.FromContext[Router](ctx)
}

type HeadlessRule interface {
	Match(metadata *InboundContext) bool
	RuleCount() uint64
	String() string
}

type Rule interface {
	HeadlessRule
	Service
	Type() string
	UpdateGeosite() error
	Outbound() string
}

type DNSRule interface {
	Rule
	DisableCache() bool
	RewriteTTL() *uint32
	ClientSubnet() *netip.Prefix
	WithAddressLimit() bool
	MatchAddressLimit(metadata *InboundContext) bool
}

type RuleSet interface {
	Name() string
	Type() string
	Format() string
	UpdatedTime() time.Time
	Update(ctx context.Context) error
	StartContext(ctx context.Context, startContext RuleSetStartContext) error
	PostStart() error
	Metadata() RuleSetMetadata
	ExtractIPSet() []*netipx.IPSet
	IncRef()
	DecRef()
	Cleanup()
	RegisterCallback(callback RuleSetUpdateCallback) *list.Element[RuleSetUpdateCallback]
	UnregisterCallback(element *list.Element[RuleSetUpdateCallback])
	Close() error
	HeadlessRule
}

type RuleSetUpdateCallback func(it RuleSet)

type RuleSetMetadata struct {
	ContainsProcessRule bool
	ContainsWIFIRule    bool
	ContainsIPCIDRRule  bool
}

type RuleSetStartContext interface {
	HTTPClient(detour string, dialer N.Dialer) *http.Client
	Close()
}

type InterfaceUpdateListener interface {
	InterfaceUpdated()
}

type WIFIState struct {
	SSID  string
	BSSID string
}
