package clashapi

import (
	"net/http"

	"github.com/sagernet/sing-box/log"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func configRouter(logFactory log.Factory) http.Handler {
	r := chi.NewRouter()
	r.Get("/", getConfigs(logFactory))
	r.Put("/", updateConfigs)
	r.Patch("/", patchConfigs)
	return r
}

type configSchema struct {
	Port        int            `json:"port"`
	SocksPort   int            `json:"socks-port"`
	RedirPort   int            `json:"redir-port"`
	TProxyPort  int            `json:"tproxy-port"`
	MixedPort   int            `json:"mixed-port"`
	AllowLan    bool           `json:"allow-lan"`
	BindAddress string         `json:"bind-address"`
	Mode        string         `json:"mode"`
	LogLevel    string         `json:"log-level"`
	IPv6        bool           `json:"ipv6"`
	Tun         map[string]any `json:"tun"`
}

func getConfigs(logFactory log.Factory) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logLevel := logFactory.Level()
		if logLevel == log.LevelTrace {
			logLevel = log.LevelDebug
		} else if logLevel < log.LevelError {
			logLevel = log.LevelError
		}
		render.JSON(w, r, &configSchema{
			Mode:        "rule",
			BindAddress: "*",
			LogLevel:    log.FormatLevel(logLevel),
		})
	}
}

func patchConfigs(w http.ResponseWriter, r *http.Request) {
	render.NoContent(w, r)
}

func updateConfigs(w http.ResponseWriter, r *http.Request) {
	render.NoContent(w, r)
}
