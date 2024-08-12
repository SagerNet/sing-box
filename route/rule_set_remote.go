package route

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

var _ adapter.RuleSet = (*RemoteRuleSet)(nil)

type RemoteRuleSet struct {
	abstractRuleSet
	ctx            context.Context
	cancel         context.CancelFunc
	path           string
	options        option.RemoteRuleSet
	updateInterval time.Duration
	dialer         N.Dialer
	lastEtag       string
	updateTicker   *time.Ticker
	pauseManager   pause.Manager
	callbackAccess sync.Mutex
	callbacks      list.List[adapter.RuleSetUpdateCallback]
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
		abstractRuleSet: abstractRuleSet{
			router: router,
			logger: logger,
			tag:    options.Tag,
			format: options.Format,
		},
		ctx:            ctx,
		cancel:         cancel,
		path:           options.Path,
		options:        options.RemoteOptions,
		updateInterval: updateInterval,
		pauseManager:   service.FromContext[pause.Manager](ctx),
	}
}

func (s *RemoteRuleSet) String() string {
	return strings.Join(F.MapToString(s.rules), " ")
}

func (s *RemoteRuleSet) StartContext(ctx context.Context, startContext adapter.RuleSetStartContext) error {
	var dialer N.Dialer
	if s.options.DownloadDetour != "" {
		outbound, loaded := s.router.Outbound(s.options.DownloadDetour)
		if !loaded {
			return E.New("download_detour not found: ", s.options.DownloadDetour)
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
	if path, err := s.getPath(s.path); err == nil {
		s.path = path
		s.loadFromFile(path)
	}
	if s.lastUpdated.IsZero() {
		err := s.fetchOnce(ctx, startContext)
		if err != nil {
			return E.Cause(err, "initial rule-set: ", s.tag)
		}
	}
	s.updateTicker = time.NewTicker(s.updateInterval)
	return nil
}

func (s *RemoteRuleSet) PostStart() error {
	go s.loopUpdate()
	return nil
}

func (s *RemoteRuleSet) RegisterCallback(callback adapter.RuleSetUpdateCallback) *list.Element[adapter.RuleSetUpdateCallback] {
	s.callbackAccess.Lock()
	defer s.callbackAccess.Unlock()
	return s.callbacks.PushBack(callback)
}

func (s *RemoteRuleSet) UnregisterCallback(element *list.Element[adapter.RuleSetUpdateCallback]) {
	s.callbackAccess.Lock()
	defer s.callbackAccess.Unlock()
	s.callbacks.Remove(element)
}

func (s *RemoteRuleSet) loadBytes(content []byte) error {
	err := s.abstractRuleSet.loadBytes(content)
	if err != nil {
		return err
	}
	s.callbackAccess.Lock()
	callbacks := s.callbacks.Array()
	s.callbackAccess.Unlock()
	for _, callback := range callbacks {
		callback(s)
	}
	return nil
}

func (s *RemoteRuleSet) loopUpdate() {
	if time.Since(s.lastUpdated) > s.updateInterval {
		s.update()
	}
	for {
		runtime.GC()
		select {
		case <-s.ctx.Done():
			return
		case <-s.updateTicker.C:
			s.pauseManager.WaitActive()
			s.update()
		}
	}
}

func (s *RemoteRuleSet) update() {
	ctx := log.ContextWithNewID(s.ctx)
	err := s.fetchOnce(ctx, nil)
	if err != nil {
		s.logger.ErrorContext(ctx, "fetch rule-set ", s.tag, ": ", err)
	} else if s.refs.Load() == 0 {
		s.rules = nil
	}
}

func (s *RemoteRuleSet) Update(ctx context.Context) error {
	err := s.fetchOnce(log.ContextWithNewID(ctx), nil)
	if err != nil {
		return err
	} else if s.refs.Load() == 0 {
		s.rules = nil
	}
	return nil
}

func (s *RemoteRuleSet) fetchOnce(ctx context.Context, startContext adapter.RuleSetStartContext) error {
	s.logger.DebugContext(ctx, "updating rule-set ", s.tag, " from URL: ", s.options.URL)
	var httpClient *http.Client
	if startContext != nil {
		httpClient = startContext.HTTPClient(s.options.DownloadDetour, s.dialer)
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
	request, err := http.NewRequest("GET", s.options.URL, nil)
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
		s.lastUpdated = time.Now()
		os.Chtimes(s.path, s.lastUpdated, s.lastUpdated)
		s.logger.InfoContext(ctx, "update rule-set ", s.tag, ": not modified")
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
	os.WriteFile(s.path, content, 0o666)
	s.logger.InfoContext(ctx, "updated rule-set ", s.tag)
	return nil
}

func (s *RemoteRuleSet) Close() error {
	s.rules = nil
	s.updateTicker.Stop()
	s.cancel()
	return nil
}
