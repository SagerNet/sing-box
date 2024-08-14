package provider

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"

	R "github.com/dlclark/regexp2"
)

var (
	_ adapter.OutboundProvider        = (*RemoteProvider)(nil)
	_ adapter.InterfaceUpdateListener = (*RemoteProvider)(nil)
)

type RemoteProvider struct {
	myProviderAdapter
	url      string
	ua       string
	interval time.Duration
	lastEtag string
	detour   string
	dialer   N.Dialer

	updateTicker     *time.Ticker
	firstStartCancel context.CancelFunc
}

func NewRemoteProvider(ctx context.Context, router adapter.Router, logger log.ContextLogger, options option.OutboundProvider, path string) (*RemoteProvider, error) {
	remoteOptions := options.RemoteOptions
	if remoteOptions.Url == "" {
		return nil, E.New("missing url")
	}
	healthcheckInterval := time.Duration(remoteOptions.HealthcheckInterval)
	healthcheckUrl := options.LocalOptions.HealthcheckUrl
	parsedURL, err := url.Parse(remoteOptions.Url)
	ua := remoteOptions.UserAgent
	downloadInterval := time.Duration(options.RemoteOptions.Interval)
	if err != nil {
		return nil, err
	}
	switch parsedURL.Scheme {
	case "":
		parsedURL.Scheme = "http"
	case "http", "https":
	default:
		return nil, E.New("invalid url scheme")
	}
	if ua == "" {
		ua = "sing-box " + C.Version + "; PuerNya fork"
	}
	if healthcheckUrl == "" {
		healthcheckUrl = "https://www.gstatic.com/generate_204"
	}
	if healthcheckInterval == 0 {
		healthcheckInterval = C.DefaultURLTestInterval
	}
	if downloadInterval < C.DefaultDonloadInterval {
		downloadInterval = C.DefaultDonloadInterval
	}
	ctx, cancel := context.WithCancel(ctx)
	provider := &RemoteProvider{
		myProviderAdapter: myProviderAdapter{
			ctx:                 ctx,
			cancel:              cancel,
			router:              router,
			logger:              logger,
			tag:                 options.Tag,
			path:                path,
			enableHealthcheck:   remoteOptions.EnableHealthcheck,
			healthcheckUrl:      healthcheckUrl,
			healthcheckInterval: healthcheckInterval,
			types:               options.Types,
			providerType:        C.ProviderTypeRemote,
			outboundOverride:    options.OutboundOverride,
			close:               make(chan struct{}),
			pauseManager:        service.FromContext[pause.Manager](ctx),
			subInfo:             SubInfo{},
			outbounds:           []adapter.Outbound{},
			outboundByTag:       make(map[string]adapter.Outbound),
		},
		url:      parsedURL.String(),
		ua:       ua,
		interval: downloadInterval,
		detour:   remoteOptions.Detour,
	}
	if len(options.Includes) > 0 {
		includes := make([]*R.Regexp, 0, len(options.Includes))
		for _, include := range options.Includes {
			regex, err := R.Compile(include, R.IgnoreCase)
			if err != nil {
				return nil, E.Cause(err, "parse includes")
			}
			includes = append(includes, regex)
		}
		provider.includes = includes
	}
	if options.Excludes != "" {
		regex, err := R.Compile(options.Excludes, R.IgnoreCase)
		if err != nil {
			return nil, E.Cause(err, "parse excludes")
		}
		provider.excludes = regex
	}
	if err := provider.firstStart(); err != nil {
		return nil, err
	}
	return provider, nil
}

func (p *RemoteProvider) PostStart() error {
	var dialer N.Dialer
	if p.detour != "" {
		outbound, loaded := p.router.Outbound(p.detour)
		if !loaded {
			return E.New("download_detour not found: ", p.detour)
		}
		dialer = outbound
	} else {
		outbound, err := p.router.DefaultOutbound(N.NetworkTCP)
		if err != nil {
			return err
		}
		dialer = outbound
	}
	p.dialer = dialer
	go p.loopUpdateCheck()
	go p.loopHealthCheck()
	return nil
}

func (p *RemoteProvider) Start() error {
	var history *urltest.HistoryStorage
	if history = service.PtrFromContext[urltest.HistoryStorage](p.ctx); history != nil {
	} else if clashServer := p.router.ClashServer(); clashServer != nil {
		history = clashServer.HistoryStorage()
	} else {
		history = urltest.NewHistoryStorage()
	}
	p.healchcheckHistory = history
	return nil
}

func (p *RemoteProvider) loopUpdateCheck() {
	p.updateTicker = time.NewTicker(p.interval)
	func() {
		if !p.lastUpdated.IsZero() && time.Since(p.lastUpdated) < p.interval {
			var ctx context.Context
			ctx, p.firstStartCancel = context.WithCancel(p.ctx)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(p.lastUpdated.Add(p.interval))):
				p.updateTicker.Reset(p.interval)
				p.firstStartCancel = nil
			}
		}
		p.pauseManager.WaitActive()
		p.updateProvider(p.ctx, p.router)
	}()
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.updateTicker.C:
			p.pauseManager.WaitActive()
			p.updateProvider(p.ctx, p.router)
		}
	}
}

func (p *RemoteProvider) updateCacheFileModTime(subInfo string) {
	info, ok := parseSubInfo(subInfo)
	if !ok {
		return
	}
	p.subInfo = info

	contentRaw := getTrimedFile(p.path)
	content := decodeBase64Safe(string(contentRaw))
	firstLine, others := getFirstLine(content)
	if _, ok := parseSubInfo(firstLine); ok {
		content = decodeBase64Safe(others)
	}
	infoStr := fmt.Sprint("# upload=", info.upload, "; download=", info.download, "; total=", info.total, "; expire=", info.expire, ";")
	content = infoStr + "\n" + content
	os.WriteFile(p.path, []byte(content), 0o666)
}

func (p *RemoteProvider) fetchOnce(ctx context.Context, router adapter.Router) error {
	defer runtime.GC()
	p.lastUpdated = time.Now()

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSHandshakeTimeout: C.TCPTimeout,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return p.dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}

	request, err := http.NewRequest("GET", p.url, nil)
	if err != nil {
		return err
	}

	if p.lastEtag != "" {
		request.Header.Set("If-None-Match", p.lastEtag)
	}

	request.Header.Set("User-Agent", p.ua)

	response, err := httpClient.Do(request)
	if err != nil {
		uErr := err.(*url.Error)
		return uErr.Err
	}

	subInfo := response.Header.Get("subscription-userinfo")

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotModified:
		p.logger.InfoContext(ctx, "update outbound provider ", p.tag, ": not modified")
		p.updateCacheFileModTime(subInfo)
		return nil
	default:
		return E.New("unexpected status: ", response.Status)
	}

	defer response.Body.Close()

	contentRaw, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if len(contentRaw) == 0 {
		return E.New("empty response")
	}

	eTagHeader := response.Header.Get("Etag")
	if eTagHeader != "" {
		p.lastEtag = eTagHeader
	}

	content := decodeBase64Safe(string(contentRaw))
	info, hasSubInfo := parseSubInfo(subInfo)

	if !hasSubInfo {
		var ok bool
		firstLine, others := getFirstLine(content)
		if info, ok = parseSubInfo(firstLine); ok {
			content = decodeBase64Safe(others)
			hasSubInfo = true
		}
	}

	updated, err := p.updateProviderFromContent(ctx, router, content)
	if err != nil {
		return err
	}

	p.subInfo = info
	p.logger.InfoContext(ctx, "update outbound provider ", p.tag, " success")

	if hasSubInfo {
		subInfo = fmt.Sprint("# upload=", info.upload, "; download=", info.download, "; total=", info.total, "; expire=", info.expire, ";")
		content = subInfo + "\n" + content
	}

	os.WriteFile(p.path, []byte(content), 0o666)

	if updated {
		p.CheckOutbounds(true)
	}
	return nil
}

func (p *RemoteProvider) UpdateProvider(ctx context.Context, router adapter.Router) error {
	if err := p.updateProvider(ctx, router); err != nil {
		return err
	}
	if p.firstStartCancel != nil {
		p.firstStartCancel()
		p.firstStartCancel = nil
	}
	p.updateTicker.Reset(p.interval)
	return nil
}

func (p *RemoteProvider) updateProvider(ctx context.Context, router adapter.Router) error {
	if p.updating.Swap(true) {
		return E.New("provider is updating")
	}
	defer p.updating.Store(false)

	ctx = log.ContextWithNewID(ctx)

	p.logger.DebugContext(ctx, "update outbound provider ", p.tag, " from network")

	if err := p.fetchOnce(ctx, router); err != nil {
		p.logger.ErrorContext(ctx, E.New("update outbound provider ", p.tag, " failed.", err))
	}

	return nil
}

func (p *RemoteProvider) Close() error {
	if p.updateTicker != nil {
		p.updateTicker.Stop()
	}
	return p.myProviderAdapter.Close()
}
