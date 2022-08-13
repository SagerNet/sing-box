package daemon

import (
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"github.com/gorilla/websocket"
)

type Options struct {
	Listen           string `json:"listen"`
	ListenPort       uint16 `json:"listen_port"`
	Secret           string `json:"secret"`
	WorkingDirectory string `json:"working_directory"`
}

type Server struct {
	options    Options
	httpServer *http.Server
	instance   Instance
}

func NewServer(options Options) *Server {
	return &Server{
		options: options,
	}
}

func (s *Server) Start() error {
	tcpConn, err := net.Listen("tcp", net.JoinHostPort(s.options.Listen, F.ToString(s.options.ListenPort)))
	if err != nil {
		return err
	}
	router := chi.NewRouter()
	router.Use(cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         300,
	}).Handler)
	if s.options.Secret != "" {
		router.Use(s.authentication)
	}
	router.Get("/ping", s.ping)
	router.Get("/status", s.status)
	router.Post("/run", s.run)
	router.Get("/stop", s.stop)
	router.Route("/debug/pprof", func(r chi.Router) {
		r.HandleFunc("/", pprof.Index)
		r.HandleFunc("/cmdline", pprof.Cmdline)
		r.HandleFunc("/profile", pprof.Profile)
		r.HandleFunc("/symbol", pprof.Symbol)
		r.HandleFunc("/trace", pprof.Trace)
	})
	httpServer := &http.Server{
		Handler: router,
	}
	go httpServer.Serve(tcpConn)
	s.httpServer = httpServer
	return nil
}

func (s *Server) authentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if websocket.IsWebSocketUpgrade(request) && request.URL.Query().Get("token") != "" {
			token := request.URL.Query().Get("token")
			if token != s.options.Secret {
				render.Status(request, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(writer, request)
			return
		}
		header := request.Header.Get("Authorization")
		bearer, token, found := strings.Cut(header, " ")
		hasInvalidHeader := bearer != "Bearer"
		hasInvalidSecret := !found || token != s.options.Secret
		if hasInvalidHeader || hasInvalidSecret {
			render.Status(request, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (s *Server) Close() error {
	return common.Close(
		common.PtrOrNil(s.httpServer),
		&s.instance,
	)
}

func (s *Server) ping(writer http.ResponseWriter, request *http.Request) {
	render.PlainText(writer, request, "pong")
}

type StatusResponse struct {
	Running bool `json:"running"`
}

func (s *Server) status(writer http.ResponseWriter, request *http.Request) {
	render.JSON(writer, request, StatusResponse{
		Running: s.instance.Running(),
	})
}

func (s *Server) run(writer http.ResponseWriter, request *http.Request) {
	err := s.run0(request)
	if err != nil {
		log.Warn(err)
		render.Status(request, http.StatusBadRequest)
		render.PlainText(writer, request, err.Error())
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (s *Server) run0(request *http.Request) error {
	configContent, err := io.ReadAll(request.Body)
	if err != nil {
		return E.Cause(err, "read config")
	}
	var options option.Options
	err = json.Unmarshal(configContent, &options)
	if err != nil {
		return E.Cause(err, "decode config")
	}
	return s.instance.Start(options)
}

func (s *Server) stop(writer http.ResponseWriter, request *http.Request) {
	s.instance.Close()
	writer.WriteHeader(http.StatusNoContent)
}
