package clashapi

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/filemanager"
	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
)

func init() {
	experimental.RegisterClashServerConstructor(NewServer)
}

var _ adapter.ClashServer = (*Server)(nil)

type Server struct {
	ctx            context.Context
	router         adapter.Router
	logger         log.Logger
	httpServer     *http.Server
	trafficManager *trafficontrol.Manager
	urlTestHistory *urltest.HistoryStorage
	mode           string
	modeList       []string
	modeUpdateHook chan<- struct{}

	externalController       bool
	externalUI               string
	externalUIDownloadURL    string
	externalUIDownloadDetour string
}

func NewServer(ctx context.Context, router adapter.Router, logFactory log.ObservableFactory, options option.ClashAPIOptions) (adapter.ClashServer, error) {
	trafficManager := trafficontrol.NewManager()
	chiRouter := chi.NewRouter()
	server := &Server{
		ctx:    ctx,
		router: router,
		logger: logFactory.NewLogger("clash-api"),
		httpServer: &http.Server{
			Addr:    options.ExternalController,
			Handler: chiRouter,
		},
		trafficManager:           trafficManager,
		modeList:                 options.ModeList,
		externalController:       options.ExternalController != "",
		externalUIDownloadURL:    options.ExternalUIDownloadURL,
		externalUIDownloadDetour: options.ExternalUIDownloadDetour,
	}
	server.urlTestHistory = service.PtrFromContext[urltest.HistoryStorage](ctx)
	if server.urlTestHistory == nil {
		server.urlTestHistory = urltest.NewHistoryStorage()
	}
	defaultMode := "Rule"
	if options.DefaultMode != "" {
		defaultMode = options.DefaultMode
	}
	if !common.Contains(server.modeList, defaultMode) {
		server.modeList = append([]string{defaultMode}, server.modeList...)
	}
	server.mode = defaultMode
	//goland:noinspection GoDeprecation
	//nolint:staticcheck
	if options.StoreMode || options.StoreSelected || options.StoreFakeIP || options.CacheFile != "" || options.CacheID != "" {
		return nil, E.New("cache_file and related fields in Clash API is deprecated in sing-box 1.8.0, use experimental.cache_file instead.")
	}
	cors := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         300,
	})
	chiRouter.Use(cors.Handler)
	chiRouter.Group(func(r chi.Router) {
		r.Use(authentication(options.Secret))
		r.Get("/", hello(options.ExternalUI != ""))
		r.Get("/logs", getLogs(logFactory))
		r.Get("/traffic", traffic(trafficManager))
		r.Get("/version", version)
		r.Mount("/configs", configRouter(server, logFactory))
		r.Mount("/proxies", proxyRouter(server, router))
		r.Mount("/rules", ruleRouter(router))
		r.Mount("/connections", connectionRouter(router, trafficManager))
		r.Mount("/providers/proxies", proxyProviderRouter())
		r.Mount("/providers/rules", ruleProviderRouter())
		r.Mount("/script", scriptRouter())
		r.Mount("/profile", profileRouter())
		r.Mount("/cache", cacheRouter(ctx))
		r.Mount("/dns", dnsRouter(router))

		server.setupMetaAPI(r)
	})
	if options.ExternalUI != "" {
		server.externalUI = filemanager.BasePath(ctx, os.ExpandEnv(options.ExternalUI))
		chiRouter.Group(func(r chi.Router) {
			fs := http.StripPrefix("/ui", http.FileServer(http.Dir(server.externalUI)))
			r.Get("/ui", http.RedirectHandler("/ui/", http.StatusTemporaryRedirect).ServeHTTP)
			r.Get("/ui/*", func(w http.ResponseWriter, r *http.Request) {
				fs.ServeHTTP(w, r)
			})
		})
	}
	return server, nil
}

func (s *Server) PreStart() error {
	cacheFile := service.FromContext[adapter.CacheFile](s.ctx)
	if cacheFile != nil {
		mode := cacheFile.LoadMode()
		if common.Any(s.modeList, func(it string) bool {
			return strings.EqualFold(it, mode)
		}) {
			s.mode = mode
		}
	}
	return nil
}

func (s *Server) Start() error {
	if s.externalController {
		s.checkAndDownloadExternalUI()
		listener, err := net.Listen("tcp", s.httpServer.Addr)
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
	s.router.ClearDNSCache()
	cacheFile := service.FromContext[adapter.CacheFile](s.ctx)
	if cacheFile != nil {
		err := cacheFile.StoreMode(newMode)
		if err != nil {
			s.logger.Error(E.Cause(err, "save mode"))
		}
	}
	s.logger.Info("updated mode: ", newMode)
}

func (s *Server) HistoryStorage() *urltest.HistoryStorage {
	return s.urlTestHistory
}

func (s *Server) TrafficManager() *trafficontrol.Manager {
	return s.trafficManager
}

func (s *Server) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule) (net.Conn, adapter.Tracker) {
	tracker := trafficontrol.NewTCPTracker(conn, s.trafficManager, castMetadata(metadata), s.router, matchedRule)
	return tracker, tracker
}

func (s *Server) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule) (N.PacketConn, adapter.Tracker) {
	tracker := trafficontrol.NewUDPTracker(conn, s.trafficManager, castMetadata(metadata), s.router, matchedRule)
	return tracker, tracker
}

func castMetadata(metadata adapter.InboundContext) trafficontrol.Metadata {
	var inbound string
	if metadata.Inbound != "" {
		inbound = metadata.InboundType + "/" + metadata.Inbound
	} else {
		inbound = metadata.InboundType
	}
	var domain string
	if metadata.Domain != "" {
		domain = metadata.Domain
	} else {
		domain = metadata.Destination.Fqdn
	}
	var processPath string
	if metadata.ProcessInfo != nil {
		if metadata.ProcessInfo.ProcessPath != "" {
			processPath = metadata.ProcessInfo.ProcessPath
		} else if metadata.ProcessInfo.PackageName != "" {
			processPath = metadata.ProcessInfo.PackageName
		}
		if processPath == "" {
			if metadata.ProcessInfo.UserId != -1 {
				processPath = F.ToString(metadata.ProcessInfo.UserId)
			}
		} else if metadata.ProcessInfo.User != "" {
			processPath = F.ToString(processPath, " (", metadata.ProcessInfo.User, ")")
		} else if metadata.ProcessInfo.UserId != -1 {
			processPath = F.ToString(processPath, " (", metadata.ProcessInfo.UserId, ")")
		}
	}
	return trafficontrol.Metadata{
		NetWork:     metadata.Network,
		Type:        inbound,
		SrcIP:       metadata.Source.Addr,
		DstIP:       metadata.Destination.Addr,
		SrcPort:     F.ToString(metadata.Source.Port),
		DstPort:     F.ToString(metadata.Destination.Port),
		Host:        domain,
		DNSMode:     "normal",
		ProcessPath: processPath,
	}
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
		if redirect {
			http.Redirect(w, r, "/ui/", http.StatusTemporaryRedirect)
		} else {
			render.JSON(w, r, render.M{"hello": "clash"})
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
		var err error
		for range tick.C {
			buf.Reset()
			up, down := trafficManager.Now()
			if err := json.NewEncoder(buf).Encode(Traffic{
				Up:   up,
				Down: down,
			}); err != nil {
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
