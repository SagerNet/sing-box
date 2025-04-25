package clashapi

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/sagernet/cors"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/filemanager"
	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func init() {
	experimental.RegisterClashServerConstructor(NewServer)
}

var _ adapter.ClashServer = (*Server)(nil)

type Server struct {
	ctx            context.Context
	router         adapter.Router
	dnsRouter      adapter.DNSRouter
	outbound       adapter.OutboundManager
	endpoint       adapter.EndpointManager
	logger         log.Logger
	httpServer     *http.Server
	trafficManager *trafficontrol.Manager
	urlTestHistory adapter.URLTestHistoryStorage
	logDebug       bool

	mode           string
	modeList       []string
	modeUpdateHook chan<- struct{}

	externalController       bool
	externalUI               string
	externalUIDownloadURL    string
	externalUIDownloadDetour string
}

func NewServer(ctx context.Context, logFactory log.ObservableFactory, options option.ClashAPIOptions) (adapter.ClashServer, error) {
	trafficManager := trafficontrol.NewManager()
	chiRouter := chi.NewRouter()
	s := &Server{
		ctx:       ctx,
		router:    service.FromContext[adapter.Router](ctx),
		dnsRouter: service.FromContext[adapter.DNSRouter](ctx),
		outbound:  service.FromContext[adapter.OutboundManager](ctx),
		endpoint:  service.FromContext[adapter.EndpointManager](ctx),
		logger:    logFactory.NewLogger("clash-api"),
		httpServer: &http.Server{
			Addr:    options.ExternalController,
			Handler: chiRouter,
		},
		trafficManager:           trafficManager,
		logDebug:                 logFactory.Level() >= log.LevelDebug,
		modeList:                 options.ModeList,
		externalController:       options.ExternalController != "",
		externalUIDownloadURL:    options.ExternalUIDownloadURL,
		externalUIDownloadDetour: options.ExternalUIDownloadDetour,
	}
	s.urlTestHistory = service.FromContext[adapter.URLTestHistoryStorage](ctx)
	if s.urlTestHistory == nil {
		s.urlTestHistory = urltest.NewHistoryStorage()
	}
	defaultMode := "Rule"
	if options.DefaultMode != "" {
		defaultMode = options.DefaultMode
	}
	if !common.Contains(s.modeList, defaultMode) {
		s.modeList = append([]string{defaultMode}, s.modeList...)
	}
	s.mode = defaultMode
	//goland:noinspection GoDeprecation
	//nolint:staticcheck
	if options.StoreMode || options.StoreSelected || options.StoreFakeIP || options.CacheFile != "" || options.CacheID != "" {
		return nil, E.New("cache_file and related fields in Clash API is deprecated in sing-box 1.8.0, use experimental.cache_file instead.")
	}
	allowedOrigins := options.AccessControlAllowOrigin
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}
	cors := cors.New(cors.Options{
		AllowedOrigins:      allowedOrigins,
		AllowedMethods:      []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders:      []string{"Content-Type", "Authorization"},
		AllowPrivateNetwork: options.AccessControlAllowPrivateNetwork,
		MaxAge:              300,
	})
	chiRouter.Use(cors.Handler)
	chiRouter.Group(func(r chi.Router) {
		r.Use(authentication(options.Secret))
		r.Get("/", hello(options.ExternalUI != ""))
		r.Get("/logs", getLogs(logFactory))
		r.Get("/traffic", traffic(trafficManager))
		r.Get("/version", version)
		r.Mount("/configs", configRouter(s, logFactory))
		r.Mount("/proxies", proxyRouter(s, s.router))
		r.Mount("/rules", ruleRouter(s.router))
		r.Mount("/connections", connectionRouter(s.router, trafficManager))
		r.Mount("/providers/proxies", proxyProviderRouter())
		r.Mount("/providers/rules", ruleProviderRouter())
		r.Mount("/script", scriptRouter())
		r.Mount("/profile", profileRouter())
		r.Mount("/cache", cacheRouter(ctx))
		r.Mount("/dns", dnsRouter(s.dnsRouter))

		s.setupMetaAPI(r)
	})
	if options.ExternalUI != "" {
		s.externalUI = filemanager.BasePath(ctx, os.ExpandEnv(options.ExternalUI))
		chiRouter.Group(func(r chi.Router) {
			r.Get("/ui", http.RedirectHandler("/ui/", http.StatusMovedPermanently).ServeHTTP)
			r.Handle("/ui/*", http.StripPrefix("/ui/", http.FileServer(Dir(s.externalUI))))
		})
	}
	return s, nil
}

func (s *Server) Name() string {
	return "clash server"
}

func (s *Server) Start(stage adapter.StartStage) error {
	switch stage {
	case adapter.StartStateStart:
		cacheFile := service.FromContext[adapter.CacheFile](s.ctx)
		if cacheFile != nil {
			mode := cacheFile.LoadMode()
			if common.Any(s.modeList, func(it string) bool {
				return strings.EqualFold(it, mode)
			}) {
				s.mode = mode
			}
		}
	case adapter.StartStateStarted:
		if s.externalController {
			s.checkAndDownloadExternalUI()
			var (
				listener net.Listener
				err      error
			)
			for i := 0; i < 3; i++ {
				listener, err = net.Listen("tcp", s.httpServer.Addr)
				if runtime.GOOS == "android" && errors.Is(err, syscall.EADDRINUSE) {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				break
			}
			if err != nil {
				return E.Cause(err, "external controller listen error")
			}
			s.logger.Info("restful api listening at ", listener.Addr())
			go func() {
				err = s.httpServer.Serve(listener)
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					s.logger.Error("external controller serve error: ", err)
				}
			}()
		}
	}

	return nil
}

func (s *Server) Close() error {
	return common.Close(
		common.PtrOrNil(s.httpServer),
		s.trafficManager,
		s.urlTestHistory,
	)
}

func (s *Server) Mode() string {
	return s.mode
}

func (s *Server) ModeList() []string {
	return s.modeList
}

func (s *Server) SetModeUpdateHook(hook chan<- struct{}) {
	s.modeUpdateHook = hook
}

func (s *Server) SetMode(newMode string) {
	if !common.Contains(s.modeList, newMode) {
		newMode = common.Find(s.modeList, func(it string) bool {
			return strings.EqualFold(it, newMode)
		})
	}
	if !common.Contains(s.modeList, newMode) {
		return
	}
	if newMode == s.mode {
		return
	}
	s.mode = newMode
	if s.modeUpdateHook != nil {
		select {
		case s.modeUpdateHook <- struct{}{}:
		default:
		}
	}
	s.dnsRouter.ClearCache()
	cacheFile := service.FromContext[adapter.CacheFile](s.ctx)
	if cacheFile != nil {
		err := cacheFile.StoreMode(newMode)
		if err != nil {
			s.logger.Error(E.Cause(err, "save mode"))
		}
	}
	s.logger.Info("updated mode: ", newMode)
}

func (s *Server) HistoryStorage() adapter.URLTestHistoryStorage {
	return s.urlTestHistory
}

func (s *Server) TrafficManager() *trafficontrol.Manager {
	return s.trafficManager
}

func (s *Server) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	return trafficontrol.NewTCPTracker(conn, s.trafficManager, metadata, s.outbound, matchedRule, matchOutbound)
}

func (s *Server) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) N.PacketConn {
	return trafficontrol.NewUDPTracker(conn, s.trafficManager, metadata, s.outbound, matchedRule, matchOutbound)
}

func authentication(serverSecret string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			if serverSecret == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Browser websocket not support custom header
			if r.Header.Get("Upgrade") == "websocket" && r.URL.Query().Get("token") != "" {
				token := r.URL.Query().Get("token")
				if token != serverSecret {
					render.Status(r, http.StatusUnauthorized)
					render.JSON(w, r, ErrUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			header := r.Header.Get("Authorization")
			bearer, token, found := strings.Cut(header, " ")

			hasInvalidHeader := bearer != "Bearer"
			hasInvalidSecret := !found || token != serverSecret
			if hasInvalidHeader || hasInvalidSecret {
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, ErrUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func hello(redirect bool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if !redirect || contentType == "application/json" {
			render.JSON(w, r, render.M{"hello": "clash"})
		} else {
			http.Redirect(w, r, "/ui/", http.StatusTemporaryRedirect)
		}
	}
}

type Traffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

func traffic(trafficManager *trafficontrol.Manager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var conn net.Conn
		if r.Header.Get("Upgrade") == "websocket" {
			var err error
			conn, _, _, err = ws.UpgradeHTTP(r, w)
			if err != nil {
				return
			}
			defer conn.Close()
		}

		if conn == nil {
			w.Header().Set("Content-Type", "application/json")
			render.Status(r, http.StatusOK)
		}

		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		buf := &bytes.Buffer{}
		uploadTotal, downloadTotal := trafficManager.Total()
		for range tick.C {
			buf.Reset()
			uploadTotalNew, downloadTotalNew := trafficManager.Total()
			err := json.NewEncoder(buf).Encode(Traffic{
				Up:   uploadTotalNew - uploadTotal,
				Down: downloadTotalNew - downloadTotal,
			})
			if err != nil {
				break
			}
			if conn == nil {
				_, err = w.Write(buf.Bytes())
				w.(http.Flusher).Flush()
			} else {
				err = wsutil.WriteServerText(conn, buf.Bytes())
			}
			if err != nil {
				break
			}

			uploadTotal = uploadTotalNew
			downloadTotal = downloadTotalNew
		}
	}
}

type Log struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

func getLogs(logFactory log.ObservableFactory) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		levelText := r.URL.Query().Get("level")
		if levelText == "" {
			levelText = "info"
		}

		level, ok := log.ParseLevel(levelText)
		if ok != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, ErrBadRequest)
			return
		}

		subscription, done, err := logFactory.Subscribe()
		if err != nil {
			render.Status(r, http.StatusNoContent)
			return
		}
		defer logFactory.UnSubscribe(subscription)

		var conn net.Conn
		if r.Header.Get("Upgrade") == "websocket" {
			conn, _, _, err = ws.UpgradeHTTP(r, w)
			if err != nil {
				return
			}
			defer conn.Close()
		}

		if conn == nil {
			w.Header().Set("Content-Type", "application/json")
			render.Status(r, http.StatusOK)
		}

		buf := &bytes.Buffer{}
		var logEntry log.Entry
		for {
			select {
			case <-done:
				return
			case logEntry = <-subscription:
			}
			if logEntry.Level > level {
				continue
			}
			buf.Reset()
			err = json.NewEncoder(buf).Encode(Log{
				Type:    log.FormatLevel(logEntry.Level),
				Payload: logEntry.Message,
			})
			if err != nil {
				break
			}
			if conn == nil {
				_, err = w.Write(buf.Bytes())
				w.(http.Flusher).Flush()
			} else {
				err = wsutil.WriteServerText(conn, buf.Bytes())
			}

			if err != nil {
				break
			}
		}
	}
}

func version(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, render.M{"version": "sing-box " + C.Version, "premium": true, "meta": true})
}
