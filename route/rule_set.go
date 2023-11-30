package route

import (
	"context"
	"net"
	"net/http"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewRuleSet(ctx context.Context, router adapter.Router, logger logger.ContextLogger, options option.RuleSet) (adapter.RuleSet, error) {
	switch options.Type {
	case C.RuleSetTypeLocal:
		return NewLocalRuleSet(router, options)
	case C.RuleSetTypeRemote:
		return NewRemoteRuleSet(ctx, router, logger, options), nil
	default:
		return nil, E.New("unknown rule set type: ", options.Type)
	}
}

var _ adapter.RuleSetStartContext = (*RuleSetStartContext)(nil)

type RuleSetStartContext struct {
	access          sync.Mutex
	httpClientCache map[string]*http.Client
}

func NewRuleSetStartContext() *RuleSetStartContext {
	return &RuleSetStartContext{
		httpClientCache: make(map[string]*http.Client),
	}
}

func (c *RuleSetStartContext) HTTPClient(detour string, dialer N.Dialer) *http.Client {
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

func (c *RuleSetStartContext) Close() {
	c.access.Lock()
	defer c.access.Unlock()
	for _, client := range c.httpClientCache {
		client.CloseIdleConnections()
	}
}
