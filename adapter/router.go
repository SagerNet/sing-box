package adapter

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/andybalholm/brotli"
	C "github.com/sagernet/sing-box/constant"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
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
	AppendTracker(tracker ConnectionTracker)
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
	ctx             context.Context
	access          sync.Mutex
	httpClientCache map[string]*http.Client
}

type MultiEncodingTransport struct {
	Base   http.RoundTripper
	Dialer func(ctx context.Context, network, addr string) (net.Conn, error)
}

func (t *MultiEncodingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Accept-Encoding", "br, gzip")

	resp, err := t.Base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		resp.Body = reader
		resp.Header.Del("Content-Encoding")
	case "br":
		resp.Body = io.NopCloser(brotli.NewReader(resp.Body))
		resp.Header.Del("Content-Encoding")
	}

	return resp, nil
}

func NewHTTPStartContext(ctx context.Context) *HTTPStartContext {
	return &HTTPStartContext{
		ctx:             ctx,
		httpClientCache: make(map[string]*http.Client),
	}
}

func (c *HTTPStartContext) HTTPClient(detour string, dialer N.Dialer) *http.Client {
	c.access.Lock()
	defer c.access.Unlock()
	if httpClient, loaded := c.httpClientCache[detour]; loaded {
		return httpClient
	}
	dialerCtx := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
	}
	httpClient := &http.Client{
		Transport: &MultiEncodingTransport{
			Base: &http.Transport{
				ForceAttemptHTTP2:   true,
				TLSHandshakeTimeout: C.TCPTimeout,
				DialContext:         dialerCtx,
				TLSClientConfig: &tls.Config{
					Time:    ntp.TimeFuncFromContext(c.ctx),
					RootCAs: RootPoolFromContext(c.ctx),
				},
			},
			Dialer: dialerCtx,
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
