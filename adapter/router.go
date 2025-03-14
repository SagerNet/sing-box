package adapter

import (
	"context"
	"net"
	"net/http"
	"sync"

	C "github.com/sagernet/sing-box/constant"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"

	"go4.org/netipx"
)

type Router interface {
	Lifecycle
	ConnectionRouter
	PreMatch(metadata InboundContext) error
	ConnectionRouterEx
	RuleSet(tag string) (RuleSet, bool)
	NeedWIFIState() bool
	Rules() []Rule
	SetTracker(tracker ConnectionTracker)
	ResetNetwork()
}

type ConnectionTracker interface {
	RoutedConnection(ctx context.Context, conn net.Conn, metadata InboundContext, matchedRule Rule, matchOutbound Outbound) net.Conn
	RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext, matchedRule Rule, matchOutbound Outbound) N.PacketConn
}

// Deprecated: Use ConnectionRouterEx instead.
type ConnectionRouter interface {
	RouteConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
}

type ConnectionRouterEx interface {
	ConnectionRouter
	RouteConnectionEx(ctx context.Context, conn net.Conn, metadata InboundContext, onClose N.CloseHandlerFunc)
	RoutePacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata InboundContext, onClose N.CloseHandlerFunc)
}

type RuleSet interface {
	Name() string
	StartContext(ctx context.Context, startContext *HTTPStartContext) error
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
type HTTPStartContext struct {
	access          sync.Mutex
	httpClientCache map[string]*http.Client
}

func NewHTTPStartContext() *HTTPStartContext {
	return &HTTPStartContext{
		httpClientCache: make(map[string]*http.Client),
	}
}

func (c *HTTPStartContext) HTTPClient(detour string, dialer N.Dialer) *http.Client {
	c.access.Lock()
	defer c.access.Unlock()
	if httpClient, loaded := c.httpClientCache[detour]; loaded {
		return httpClient
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2:   true,
			TLSHandshakeTimeout: C.TCPTimeout,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}
	c.httpClientCache[detour] = httpClient
	return httpClient
}

func (c *HTTPStartContext) Close() {
	c.access.Lock()
	defer c.access.Unlock()
	for _, client := range c.httpClientCache {
		client.CloseIdleConnections()
	}
}
