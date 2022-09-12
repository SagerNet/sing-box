package outbound

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

type Provider struct {
	url      string
	filter   *regexp2.Regexp
	interval time.Duration
	logger   log.ContextLogger
	ctx      context.Context
	router   adapter.Router
}

type providerAdapter struct {
	providers []Provider
}

type CachedProvider struct {
	Outbounds  []adapter.Outbound
	LastUpdate time.Time
	Lock       *sync.Mutex
}

type ProviderResolver interface {
	GetOutbounds([]byte, context.Context, adapter.Router, log.ContextLogger) []adapter.Outbound
}

var (
	providerResolvers = make(map[string]ProviderResolver)
	cachedProviders   = make(map[string]*CachedProvider, 0)
	compatibleProxy   adapter.Outbound
)

func NewProvider(url, filterStr string, interval int, ctx context.Context, router adapter.Router,
	logger log.ContextLogger) (Provider, error) {
	if interval == 0 {
		interval = 3600 * 24
	}
	filter, err := regexp2.Compile(filterStr, 0)
	if err != nil {
		return Provider{}, E.New("cannot parse provider regex filter")
	}
	return Provider{
			url:      url,
			filter:   filter,
			interval: time.Duration(interval) * time.Second,
			ctx:      ctx,
			router:   router,
			logger:   logger},
		nil
}

func (p *Provider) GetOutbounds() ([]string, map[string]adapter.Outbound) {
	tags := make([]string, 0)
	outbounds := make(map[string]adapter.Outbound, 0)
	allOutbounds := p.getAllOutbounds()
	for _, outbound := range allOutbounds {
		if ok, _ := p.filter.MatchString(outbound.Tag()); ok {
			p.router.AddOutbound(outbound.Tag(), outbound)
			tags = append(tags, outbound.Tag())
			outbounds[outbound.Tag()] = outbound
		}
	}
	return tags, outbounds
}

func (p *Provider) getAllOutbounds() (res []adapter.Outbound) {
	defer func() {
		if r := recover(); r != nil {
			res = make([]adapter.Outbound, 0)
			p.logger.Warn("failed to get provider outbounds: ", r)
		}
	}()
	if _, ok := cachedProviders[p.url]; !ok {
		cachedProviders[p.url] = &CachedProvider{
			Outbounds:  make([]adapter.Outbound, 0),
			LastUpdate: time.Time{},
			Lock:       &sync.Mutex{},
		}
	}
	cachedProviders[p.url].Lock.Lock()
	defer cachedProviders[p.url].Lock.Unlock()
	if (cachedProviders[p.url].LastUpdate.Add(p.interval)).Before(time.Now()) {
		outbounds := make([]adapter.Outbound, 0)
		resp, err := http.DefaultClient.Get(p.url)
		if err == nil {
			body := resp.Body
			defer body.Close()
			content, _ := io.ReadAll(body)
			for _, resolver := range providerResolvers {
				if len(outbounds) == 0 {
					outbounds = resolver.GetOutbounds(content, p.ctx, p.router, p.logger)
				}
			}
			cachedProviders[p.url].SetOutbounds(outbounds)
		}
	}
	cachedProviders[p.url].RefeshUpdateTime()
	return cachedProviders[p.url].Outbounds
}

func (p *providerAdapter) NewUpdateFunc(tags *[]string, outbounds *map[string]adapter.Outbound, router adapter.Router, funcs []func()) func() {
	res := func() {
		for _, f := range funcs {
			defer f()
		}
		for _, provider := range p.providers {
			_, newOutbounds := provider.GetOutbounds()
			for k, v := range newOutbounds {
				if _, ok := (*outbounds)[k]; !ok {
					*tags = append(*tags, k)
					(*outbounds)[k] = v
				}
			}
		}
		if len(*tags) == 0 {
			p.AddCompatibleProxy(tags, outbounds, router)
		}
		if len(*tags) > 2 {
			if _, ok := (*outbounds)["compatible"]; ok {
				delete(*outbounds, "compatible")
				for i, tag := range *tags {
					if tag == "compatible" {
						*tags = append((*tags)[:i], (*tags)[i+1:]...)
						break
					}
				}
			}
		}
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			res()
		}
	}()
	return res
}

func (p *providerAdapter) AddCompatibleProxy(tags *[]string, outbounds *map[string]adapter.Outbound, router adapter.Router) {
	if len(*tags) == 0 {
		*tags = append(*tags, "compatible")
		(*outbounds)["compatible"] = compatibleProxy
		router.AddOutbound("compatible", compatibleProxy)
	}
}

func (p *CachedProvider) SetOutbounds(outbounds []adapter.Outbound) {
	p.Outbounds = outbounds
}

func (p *CachedProvider) RefeshUpdateTime() {
	p.LastUpdate = time.Now()
}

func InjectClashProviderResolver(name string, resolver ProviderResolver) {
	providerResolvers[name] = resolver
}

func InitCompatibleProxy(router adapter.Router, logger log.ContextLogger, opts option.DirectOutboundOptions) adapter.Outbound {
	var err error
	compatibleProxy, err = NewDirect(router, logger, "compatible", opts)
	if err != nil {
		logger.Panic("cannot create compatible proxy")
	}
	return compatibleProxy
}
