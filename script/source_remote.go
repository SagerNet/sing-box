package script

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"

	"github.com/dop251/goja"
)

var _ Source = (*RemoteSource)(nil)

type RemoteSource struct {
	ctx            context.Context
	cancel         context.CancelFunc
	logger         logger.Logger
	outbound       adapter.OutboundManager
	options        option.Script
	updateInterval time.Duration
	dialer         N.Dialer
	program        *goja.Program
	lastUpdated    time.Time
	lastEtag       string
	updateTicker   *time.Ticker
	cacheFile      adapter.CacheFile
	pauseManager   pause.Manager
}

func NewRemoteSource(ctx context.Context, logger logger.Logger, options option.Script) (*RemoteSource, error) {
	ctx, cancel := context.WithCancel(ctx)
	var updateInterval time.Duration
	if options.RemoteOptions.UpdateInterval > 0 {
		updateInterval = time.Duration(options.RemoteOptions.UpdateInterval)
	} else {
		updateInterval = 24 * time.Hour
	}
	return &RemoteSource{
		ctx:            ctx,
		cancel:         cancel,
		logger:         logger,
		outbound:       service.FromContext[adapter.OutboundManager](ctx),
		options:        options,
		updateInterval: updateInterval,
		pauseManager:   service.FromContext[pause.Manager](ctx),
	}, nil
}

func (s *RemoteSource) StartContext(ctx context.Context, startContext *adapter.HTTPStartContext) error {
	s.cacheFile = service.FromContext[adapter.CacheFile](s.ctx)
	var dialer N.Dialer
	if s.options.RemoteOptions.DownloadDetour != "" {
		outbound, loaded := s.outbound.Outbound(s.options.RemoteOptions.DownloadDetour)
		if !loaded {
			return E.New("download detour not found: ", s.options.RemoteOptions.DownloadDetour)
		}
		dialer = outbound
	} else {
		dialer = s.outbound.Default()
	}
	s.dialer = dialer
	if s.cacheFile != nil {
		if savedSet := s.cacheFile.LoadScript(s.options.Tag); savedSet != nil {
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
			return E.Cause(err, "initial rule-set: ", s.options.Tag)
		}
	}
	s.updateTicker = time.NewTicker(s.updateInterval)
	return nil
}

func (s *RemoteSource) PostStart() error {
	go s.loopUpdate()
	return nil
}

func (s *RemoteSource) Program() *goja.Program {
	return s.program
}

func (s *RemoteSource) loadBytes(content []byte) error {
	program, err := goja.Compile(F.ToString("script:", s.options.Tag), string(content), false)
	if err != nil {
		return err
	}
	s.program = program
	return nil
}

func (s *RemoteSource) loopUpdate() {
	if time.Since(s.lastUpdated) > s.updateInterval {
		err := s.fetchOnce(s.ctx, nil)
		if err != nil {
			s.logger.Error("fetch rule-set ", s.options.Tag, ": ", err)
		}
	}
	for {
		runtime.GC()
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

func (s *RemoteSource) fetchOnce(ctx context.Context, startContext *adapter.HTTPStartContext) error {
	s.logger.Debug("updating script ", s.options.Tag, " from URL: ", s.options.RemoteOptions.URL)
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
				TLSClientConfig: &tls.Config{
					Time:    ntp.TimeFuncFromContext(s.ctx),
					RootCAs: adapter.RootPoolFromContext(s.ctx),
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
		s.lastUpdated = time.Now()
		if s.cacheFile != nil {
			savedRuleSet := s.cacheFile.LoadScript(s.options.Tag)
			if savedRuleSet != nil {
				savedRuleSet.LastUpdated = s.lastUpdated
				err = s.cacheFile.SaveScript(s.options.Tag, savedRuleSet)
				if err != nil {
					s.logger.Error("save script updated time: ", err)
					return nil
				}
			}
		}
		s.logger.Info("update script ", s.options.Tag, ": not modified")
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
	if s.cacheFile != nil {
		err = s.cacheFile.SaveScript(s.options.Tag, &adapter.SavedBinary{
			LastUpdated: s.lastUpdated,
			Content:     content,
			LastEtag:    s.lastEtag,
		})
		if err != nil {
			s.logger.Error("save script cache: ", err)
		}
	}
	s.logger.Info("updated script ", s.options.Tag)
	return nil
}

func (s *RemoteSource) Close() error {
	if s.updateTicker != nil {
		s.updateTicker.Stop()
	}
	s.cancel()
	return nil
}
