package service

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
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
	options    option.SubscriptionServiceOptions

	ctx    context.Context
	cancel context.CancelFunc
}

// NewSubscription creates a new subscription service.
func NewSubscription(ctx context.Context, router adapter.Router, logger log.ContextLogger, logFactory log.Factory, options option.Service) (*Subscription, error) {
	ctx2, cancel := context.WithCancel(ctx)
	return &Subscription{
		myServiceAdapter: myServiceAdapter{
			router:      router,
			serviceType: C.ServiceSubscription,
			logger:      logger,
			tag:         options.Tag,
		},
		options:    options.SubscriptionOptions,
		parentCtx:  ctx,
		logFactory: logFactory,
		ctx:        ctx2,
		cancel:     cancel,
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
	ticker := time.NewTicker(time.Duration(s.options.Interval))
	defer ticker.Stop()

	if err := s.fetch(); err != nil {
		s.logger.Error("fetch subscription: ", err)
	}
	for {
		select {
		case <-s.ctx.Done():
			break
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
	for i, provider := range s.options.Providers {
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
			// TODO: filter links
			opt := link.Options()
			s.applyOptions(opt, &provider)
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

func (s *Subscription) applyOptions(options *option.Outbound, provider *option.SubscriptionProviderOptions) {
	options.Tag = s.tag + provider.Tag + options.Tag
	// TODO: implement me
}

func (s *Subscription) fetchProvider(client *http.Client, provider option.SubscriptionProviderOptions) ([]link.Link, error) {
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
	// try undecoded
	content := string(body)
	links, err := linksFromContent(content)
	if len(links) > 0 {
		if err != nil {
			s.logger.Warn("some links failed", err)
		}
		return links, nil
	}
	// try decoded
	decoded, err := base64Decode(content)
	links, err = linksFromContent(string(decoded))
	if len(links) > 0 {
		if err != nil {
			s.logger.Warn("some links failed", err)
		}
		return links, nil
	}
	return nil, E.New("no links found")
}

func linksFromContent(content string) ([]link.Link, error) {
	links := make([]link.Link, 0)
	errs := make([]error, 0)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		u, err := url.Parse(line)
		if err != nil {
			continue
		}
		link, err := link.Parse(u)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		links = append(links, link)
	}
	return links, E.Errors(errs...)
}

func base64Decode(b64 string) ([]byte, error) {
	b64 = strings.TrimSpace(b64)
	stdb64 := b64
	if pad := len(b64) % 4; pad != 0 {
		stdb64 += strings.Repeat("=", 4-pad)
	}

	b, err := base64.StdEncoding.DecodeString(stdb64)
	if err != nil {
		return base64.URLEncoding.DecodeString(b64)
	}
	return b, nil
}

func (s *Subscription) client() (*http.Client, error) {
	var detour adapter.Outbound
	if s.options.DownloadDetour != "" {
		outbound, loaded := s.router.Outbound(s.options.DownloadDetour)
		if !loaded {
			return nil, E.New("detour outbound not found: ", s.options.DownloadDetour)
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
