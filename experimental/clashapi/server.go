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
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	N "github.com/sagernet/sing/common/network"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"github.com/goccy/go-json"
	"github.com/gorilla/websocket"
)

var _ adapter.ClashServer = (*Server)(nil)

type Server struct {
	router         adapter.Router
	logger         log.Logger
	httpServer     *http.Server
	trafficManager *trafficontrol.Manager
}

func NewServer(router adapter.Router, logFactory log.ObservableFactory, options option.ClashAPIOptions) *Server {
	trafficManager := trafficontrol.NewManager()
	chiRouter := chi.NewRouter()
	server := &Server{
		router,
		logFactory.NewLogger("clash-api"),
		&http.Server{
			Addr:    options.ExternalController,
			Handler: chiRouter,
		},
		trafficManager,
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
		r.Get("/", hello)
		r.Get("/logs", getLogs(logFactory))
		r.Get("/traffic", traffic(trafficManager))
		r.Get("/version", version)
		r.Mount("/configs", configRouter(logFactory))
		r.Mount("/proxies", proxyRouter(server, router))
		r.Mount("/rules", ruleRouter(router))
		r.Mount("/connections", connectionRouter(trafficManager))
		r.Mount("/providers/proxies", proxyProviderRouter())
		r.Mount("/providers/rules", ruleProviderRouter())
		r.Mount("/script", scriptRouter())
		r.Mount("/profile", profileRouter())
		r.Mount("/cache", cacheRouter())
	})
	if options.ExternalUI != "" {
		chiRouter.Group(func(r chi.Router) {
			fs := http.StripPrefix("/ui", http.FileServer(http.Dir(os.ExpandEnv(options.ExternalUI))))
			r.Get("/ui", http.RedirectHandler("/ui/", http.StatusTemporaryRedirect).ServeHTTP)
			r.Get("/ui/*", func(w http.ResponseWriter, r *http.Request) {
				fs.ServeHTTP(w, r)
			})
		})
	}
	return server
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
	return s.httpServer.Close()
}

func (s *Server) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule) net.Conn {
	return trafficontrol.NewTCPTracker(conn, s.trafficManager, castMetadata(metadata), s.router, matchedRule)
}

func (s *Server) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule) N.PacketConn {
	return trafficontrol.NewUDPTracker(conn, s.trafficManager, castMetadata(metadata), s.router, matchedRule)
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
	return trafficontrol.Metadata{
		NetWork: metadata.Network,
		Type:    inbound,
		SrcIP:   metadata.Source.Addr,
		DstIP:   metadata.Destination.Addr,
		SrcPort: F.ToString(metadata.Source.Port),
		DstPort: F.ToString(metadata.Destination.Port),
		Host:    domain,
		DNSMode: "normal",
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

func hello(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, render.M{"hello": "clash"})
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
