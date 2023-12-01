package route

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

var _ adapter.RuleSet = (*RemoteRuleSet)(nil)

type RemoteRuleSet struct {
	ctx            context.Context
	cancel         context.CancelFunc
	router         adapter.Router
	logger         logger.ContextLogger
	options        option.RuleSet
	updateInterval time.Duration
	dialer         N.Dialer
	rules          []adapter.HeadlessRule
	lastUpdated    time.Time
	lastEtag       string
	updateTicker   *time.Ticker
	pauseManager   pause.Manager
}

func NewRemoteRuleSet(ctx context.Context, router adapter.Router, logger logger.ContextLogger, options option.RuleSet) *RemoteRuleSet {
	ctx, cancel := context.WithCancel(ctx)
	var updateInterval time.Duration
	if options.RemoteOptions.UpdateInterval > 0 {
		updateInterval = time.Duration(options.RemoteOptions.UpdateInterval)
	} else {
		updateInterval = 24 * time.Hour
	}
	return &RemoteRuleSet{
		ctx:            ctx,
		cancel:         cancel,
		router:         router,
		logger:         logger,
		options:        options,
		updateInterval: updateInterval,
		pauseManager:   pause.ManagerFromContext(ctx),
	}
}

func (s *RemoteRuleSet) Match(metadata *adapter.InboundContext) bool {
	for _, rule := range s.rules {
		if rule.Match(metadata) {
			return true
		}
	}
	return false
}

func (s *RemoteRuleSet) StartContext(ctx context.Context, startContext adapter.RuleSetStartContext) error {
	var dialer N.Dialer
	if s.options.RemoteOptions.DownloadDetour != "" {
		outbound, loaded := s.router.Outbound(s.options.RemoteOptions.DownloadDetour)
		if !loaded {
			return E.New("download_detour not found: ", s.options.RemoteOptions.DownloadDetour)
		}
		dialer = outbound
	} else {
		outbound, err := s.router.DefaultOutbound(N.NetworkTCP)
		if err != nil {
			return err
		}
		dialer = outbound
	}
	s.dialer = dialer
	cacheFile := service.FromContext[adapter.CacheFile](s.ctx)
	if cacheFile != nil {
		if savedSet := cacheFile.LoadRuleSet(s.options.Tag); savedSet != nil {
			err := s.loadBytes(savedSet.Content)
			if err != nil {
				return E.Cause(err, "restore cached rule-set")
			}
			s.lastUpdated = savedSet.LastUpdated
			s.lastEtag = savedSet.LastEtag
		}
	}
	if s.lastUpdated.IsZero() {
		err := s.fetchOnce(ctx, startContext)
		if err != nil {
			return E.Cause(err, "fetch rule-set ", s.options.Tag)
		}
	}
	s.updateTicker = time.NewTicker(s.updateInterval)
	go s.loopUpdate()
	return nil
}

func (s *RemoteRuleSet) loadBytes(content []byte) error {
	var (
		plainRuleSet option.PlainRuleSet
		err          error
	)
	switch s.options.Format {
	case C.RuleSetFormatSource, "":
		var compat option.PlainRuleSetCompat
		decoder := json.NewDecoder(json.NewCommentFilter(bytes.NewReader(content)))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&compat)
		if err != nil {
			return err
		}
		plainRuleSet = compat.Upgrade()
	case C.RuleSetFormatBinary:
		plainRuleSet, err = srs.Read(bytes.NewReader(content), false)
		if err != nil {
			return err
		}
	default:
		return E.New("unknown rule set format: ", s.options.Format)
	}
	rules := make([]adapter.HeadlessRule, len(plainRuleSet.Rules))
	for i, ruleOptions := range plainRuleSet.Rules {
		rules[i], err = NewHeadlessRule(s.router, ruleOptions)
		if err != nil {
			return E.Cause(err, "parse rule_set.rules.[", i, "]")
		}
	}
	s.rules = rules
	return nil
}

func (s *RemoteRuleSet) loopUpdate() {
	if time.Since(s.lastUpdated) > s.updateInterval {
		err := s.fetchOnce(s.ctx, nil)
		if err != nil {
			s.logger.Error("fetch rule-set ", s.options.Tag, ": ", err)
		}
	}
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.updateTicker.C:
			s.pauseManager.WaitActive()
			err := s.fetchOnce(s.ctx, nil)
			if err != nil {
				s.logger.Error("fetch rule-set ", s.options.Tag, ": ", err)
			}
		}
	}
}

func (s *RemoteRuleSet) fetchOnce(ctx context.Context, startContext adapter.RuleSetStartContext) error {
	s.logger.Debug("updating rule-set ", s.options.Tag, " from URL: ", s.options.RemoteOptions.URL)
	var httpClient *http.Client
	if startContext != nil {
		httpClient = startContext.HTTPClient(s.options.RemoteOptions.DownloadDetour, s.dialer)
	} else {
		httpClient = &http.Client{
			Transport: &http.Transport{
				ForceAttemptHTTP2:   true,
				TLSHandshakeTimeout: C.TCPTimeout,
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return s.dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
				},
			},
		}
	}
	request, err := http.NewRequest("GET", s.options.RemoteOptions.URL, nil)
	if err != nil {
		return err
	}
	if s.lastEtag != "" {
		request.Header.Set("If-None-Match", s.lastEtag)
	}
	response, err := httpClient.Do(request.WithContext(ctx))
	if err != nil {
		return err
	}
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotModified:
		s.logger.Info("update rule-set ", s.options.Tag, ": not modified")
		return nil
	default:
		return E.New("unexpected status: ", response.Status)
	}
	content, err := io.ReadAll(response.Body)
	if err != nil {
		response.Body.Close()
		return err
	}
	err = s.loadBytes(content)
	if err != nil {
		response.Body.Close()
		return err
	}
	response.Body.Close()
	eTagHeader := response.Header.Get("Etag")
	if eTagHeader != "" {
		s.lastEtag = eTagHeader
	}
	s.lastUpdated = time.Now()
	cacheFile := service.FromContext[adapter.CacheFile](s.ctx)
	if cacheFile != nil {
		err = cacheFile.SaveRuleSet(s.options.Tag, &adapter.SavedRuleSet{
			LastUpdated: s.lastUpdated,
			Content:     content,
			LastEtag:    s.lastEtag,
		})
		if err != nil {
			s.logger.Error("save rule-set cache: ", err)
		}
	}
	s.logger.Info("updated rule-set ", s.options.Tag)
	return nil
}

func (s *RemoteRuleSet) Close() error {
	s.updateTicker.Stop()
	s.cancel()
	return nil
}
