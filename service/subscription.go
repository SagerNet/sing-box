package service

import (
	"context"
	"io"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/link"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/outbound"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.BoxService = (*Subscription)(nil)

// Subscription is a service that subscribes to remote servers for outbounds.
type Subscription struct {
	myServiceAdapter

	parentCtx  context.Context
	logFactory log.Factory

	interval       time.Duration
	downloadDetour string `json:"download_detour,omitempty"`
	dialerOptions  option.DialerOptions
	providers      []*subscriptionProvider

	ctx    context.Context
	cancel context.CancelFunc
}

type subscriptionProvider struct {
	Tag     string         `json:"tag,omitempty"`
	URL     string         `json:"url"`
	Exclude *regexp.Regexp `json:"exclude,omitempty"`
	Include *regexp.Regexp `json:"include,omitempty"`
}

// NewSubscription creates a new subscription service.
func NewSubscription(ctx context.Context, router adapter.Router, logger log.ContextLogger, logFactory log.Factory, options option.Service) (*Subscription, error) {
	ctx2, cancel := context.WithCancel(ctx)
	providers := make([]*subscriptionProvider, 0, len(options.SubscriptionOptions.Providers))
	for _, p := range options.SubscriptionOptions.Providers {
		var (
			err     error
			exclude *regexp.Regexp
			include *regexp.Regexp
		)
		if p.Exclude != "" {
			exclude, err = regexp.Compile(p.Exclude)
			if err != nil {
				return nil, err
			}
		}
		if p.Include != "" {
			include, err = regexp.Compile(p.Include)
			if err != nil {
				return nil, err
			}
		}
		providers = append(providers, &subscriptionProvider{
			Tag:     p.Tag,
			URL:     p.URL,
			Exclude: exclude,
			Include: include,
		})
	}

	return &Subscription{
		myServiceAdapter: myServiceAdapter{
			router:      router,
			serviceType: C.ServiceSubscription,
			logger:      logger,
			tag:         options.Tag,
		},
		interval:       time.Duration(options.SubscriptionOptions.Interval),
		downloadDetour: options.SubscriptionOptions.DownloadDetour,
		providers:      providers,
		dialerOptions:  options.SubscriptionOptions.DialerOptions,
		parentCtx:      ctx,
		logFactory:     logFactory,
		ctx:            ctx2,
		cancel:         cancel,
	}, nil
}

// Start starts the service.
func (s *Subscription) Start() error {
	go s.fetchLoop()
	return nil
}

// Close closes the service.
func (s *Subscription) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *Subscription) fetchLoop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	if err := s.fetch(); err != nil {
		s.logger.Error("fetch subscription: ", err)
	}
L:
	for {
		select {
		case <-s.ctx.Done():
			break L
		case <-ticker.C:
			if err := s.fetch(); err != nil {
				s.logger.Error("fetch subscription: ", err)
			}
		}
	}
}

func (s *Subscription) fetch() error {
	client, err := s.client()
	if err != nil {
		return err
	}
	for i, provider := range s.providers {
		var tag string
		if provider.Tag != "" {
			tag = provider.Tag
		} else {
			tag = F.ToString(i)
		}
		links, err := s.fetchProvider(client, provider)
		if err != nil {
			s.logger.Warn("fetch provider [", tag, "]: ", err)
			continue
		}
		s.logger.Info(len(links), " links found from provider [", tag, "]")
		for _, link := range links {
			opt := link.Options()
			if !applyFilter(opt.Tag, provider) {
				continue
			}
			s.applyOptions(opt, provider)
			outbound, err := outbound.New(
				s.parentCtx,
				s.router,
				s.logFactory.NewLogger(F.ToString("outbound/", opt.Type, "[", opt.Tag, "]")),
				*opt,
			)
			if err != nil {
				s.logger.Warn("create outbound [", opt.Tag, "]: ", err)
			}
			s.router.AddOutbound(outbound)
			s.logger.Info("created outbound [", opt.Tag, "]")
		}
	}
	return nil
}

func applyFilter(tag string, provider *subscriptionProvider) bool {
	if provider.Exclude != nil && provider.Exclude.MatchString(tag) {
		return false
	}
	if provider.Include == nil {
		return true
	}
	return provider.Include.MatchString(tag)
}

func (s *Subscription) applyOptions(options *option.Outbound, provider *subscriptionProvider) error {
	options.Tag = s.tag + "." + provider.Tag + "." + options.Tag
	switch options.Type {
	case C.TypeSocks:
		options.SocksOptions.DialerOptions = s.dialerOptions
	case C.TypeHTTP:
		options.HTTPOptions.DialerOptions = s.dialerOptions
	case C.TypeShadowsocks:
		options.ShadowsocksOptions.DialerOptions = s.dialerOptions
	case C.TypeVMess:
		options.VMessOptions.DialerOptions = s.dialerOptions
	case C.TypeTrojan:
		options.TrojanOptions.DialerOptions = s.dialerOptions
	case C.TypeWireGuard:
		options.WireGuardOptions.DialerOptions = s.dialerOptions
	case C.TypeHysteria:
		options.HysteriaOptions.DialerOptions = s.dialerOptions
	case C.TypeTor:
		options.TorOptions.DialerOptions = s.dialerOptions
	case C.TypeSSH:
		options.SSHOptions.DialerOptions = s.dialerOptions
	case C.TypeShadowTLS:
		options.ShadowTLSOptions.DialerOptions = s.dialerOptions
	default:
		return E.New("unknown outbound type: ", options.Type)
	}
	return nil
}

func (s *Subscription) fetchProvider(client *http.Client, provider *subscriptionProvider) ([]link.Link, error) {
	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, provider.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, E.New("unexpected status code: ", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	links, err := link.ParseCollection(string(body))
	if len(links) > 0 {
		if err != nil {
			s.logger.Warn("links parsed with error:", err)
		}
		return links, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, E.New("no links found")
}

func (s *Subscription) client() (*http.Client, error) {
	var detour adapter.Outbound
	if s.downloadDetour != "" {
		outbound, loaded := s.router.Outbound(s.downloadDetour)
		if !loaded {
			return nil, E.New("detour outbound not found: ", s.downloadDetour)
		}
		detour = outbound
	} else {
		detour = s.router.DefaultOutbound(N.NetworkTCP)
	}
	return &http.Client{
		Timeout: time.Second * 30,
		Transport: &http.Transport{
			ForceAttemptHTTP2:   true,
			TLSHandshakeTimeout: 5 * time.Second,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return detour.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}, nil
}
