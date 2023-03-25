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
	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/experimental/clashapi/cachefile"
	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/websocket"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
)

func init() {
	experimental.RegisterClashServerConstructor(NewServer)
}

var _ adapter.ClashServer = (*Server)(nil)

type Server struct {
	router         adapter.Router
	logger         log.Logger
	httpServer     *http.Server
	trafficManager *trafficontrol.Manager
	urlTestHistory *urltest.HistoryStorage
	mode           string
	storeSelected  bool
	storeFakeIP    bool
	cacheFilePath  string
	cacheFile      adapter.ClashCacheFile
}

func NewServer(router adapter.Router, logFactory log.ObservableFactory, options option.ClashAPIOptions) (adapter.ClashServer, error) {
	trafficManager := trafficontrol.NewManager()
	chiRouter := chi.NewRouter()
	server := &Server{
		router: router,
		logger: logFactory.NewLogger("clash-api"),
		httpServer: &http.Server{
			Addr:    options.ExternalController,
			Handler: chiRouter,
		},
		trafficManager: trafficManager,
		urlTestHistory: urltest.NewHistoryStorage(),
		mode:           strings.ToLower(options.DefaultMode),
		storeSelected:  options.StoreSelected,
		storeFakeIP:    options.StoreFakeIP,
	}
	if server.mode == "" {
		server.mode = "rule"
	}
	if options.StoreSelected || options.StoreFakeIP {
		cachePath := os.ExpandEnv(options.CacheFile)
		if cachePath == "" {
			cachePath = "cache.db"
		}
		if foundPath, loaded := C.FindPath(cachePath); loaded {
			cachePath = foundPath
		} else {
			cachePath = C.BasePath(cachePath)
		}
		server.cacheFilePath = cachePath
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
		r.Mount("/configs", configRouter(server, logFactory, server.logger))
		r.Mount("/proxies", proxyRouter(server, router))
		r.Mount("/rules", ruleRouter(router))
		r.Mount("/connections", connectionRouter(trafficManager))
		r.Mount("/providers/proxies", proxyProviderRouter())
		r.Mount("/providers/rules", ruleProviderRouter())
		r.Mount("/script", scriptRouter())
		r.Mount("/profile", profileRouter())
		r.Mount("/cache", cacheRouter(router))
		r.Mount("/dns", dnsRouter(router))
	})
	if options.ExternalUI != "" {
		chiRouter.Group(func(r chi.Router) {
			fs := http.StripPrefix("/ui", http.FileServer(http.Dir(C.BasePath(os.ExpandEnv(options.ExternalUI)))))
			r.Get("/ui", http.RedirectHandler("/ui/", http.StatusTemporaryRedirect).ServeHTTP)
			r.Get("/ui/*", func(w http.ResponseWriter, r *http.Request) {
				fs.ServeHTTP(w, r)
			})
		})
	}
	return server, nil
}

func (s *Server) PreStart() error {
	if s.cacheFilePath != "" {
		cacheFile, err := cachefile.Open(s.cacheFilePath)
		if err != nil {
			return E.Cause(err, "open cache file")
		}
		s.cacheFile = cacheFile
	}
	return nil
}

func (s *Server) Start() error {
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
	return nil
}

func (s *Server) Close() error {
	return common.Close(
		common.PtrOrNil(s.httpServer),
		s.trafficManager,
		s.cacheFile,
	)
}

func (s *Server) Mode() string {
	return s.mode
}

func (s *Server) StoreSelected() bool {
	return s.storeSelected
}

func (s *Server) StoreFakeIP() bool {
	return s.storeFakeIP
}

func (s *Server) CacheFile() adapter.ClashCacheFile {
	return s.cacheFile
}

func (s *Server) HistoryStorage() *urltest.HistoryStorage {
	return s.urlTestHistory
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
			if websocket.IsWebSocketUpgrade(r) && r.URL.Query().Get("token") != "" {
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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Traffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

func traffic(trafficManager *trafficontrol.Manager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var wsConn *websocket.Conn
		if websocket.IsWebSocketUpgrade(r) {
			var err error
			wsConn, err = upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
		}

		if wsConn == nil {
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

			if wsConn == nil {
				_, err = w.Write(buf.Bytes())
				w.(http.Flusher).Flush()
			} else {
				err = wsConn.WriteMessage(websocket.TextMessage, buf.Bytes())
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

		var wsConn *websocket.Conn
		if websocket.IsWebSocketUpgrade(r) {
			var err error
			wsConn, err = upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
		}

		if wsConn == nil {
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
			if wsConn == nil {
				_, err = w.Write(buf.Bytes())
				w.(http.Flusher).Flush()
			} else {
				err = wsConn.WriteMessage(websocket.TextMessage, buf.Bytes())
			}

			if err != nil {
				break
			}
		}
	}
}

func version(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, render.M{"version": "sing-box " + C.Version, "premium": true})
}
