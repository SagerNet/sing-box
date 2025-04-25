package clashapi

import (
	"bytes"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

// API created by Clash.Meta

func (s *Server) setupMetaAPI(r chi.Router) {
	if s.logDebug {
		r := chi.NewRouter()
		r.Put("/gc", func(w http.ResponseWriter, r *http.Request) {
			debug.FreeOSMemory()
		})
		r.Mount("/", middleware.Profiler())
	}
	r.Get("/memory", memory(s.trafficManager))
	r.Mount("/group", groupRouter(s))
	r.Mount("/upgrade", upgradeRouter(s))
}

type Memory struct {
	Inuse   uint64 `json:"inuse"`
	OSLimit uint64 `json:"oslimit"` // maybe we need it in the future
}

func memory(trafficManager *trafficontrol.Manager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var conn net.Conn
		if r.Header.Get("Upgrade") == "websocket" {
			var err error
			conn, _, _, err = ws.UpgradeHTTP(r, w)
			if err != nil {
				return
			}
		}

		if conn == nil {
			w.Header().Set("Content-Type", "application/json")
			render.Status(r, http.StatusOK)
		}

		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		buf := &bytes.Buffer{}
		var err error
		first := true
		for range tick.C {
			buf.Reset()

			inuse := trafficManager.Snapshot().Memory

			// make chat.js begin with zero
			// this is shit var,but we need output 0 for first time
			if first {
				first = false
				inuse = 0
			}
			if err := json.NewEncoder(buf).Encode(Memory{
				Inuse:   inuse,
				OSLimit: 0,
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
