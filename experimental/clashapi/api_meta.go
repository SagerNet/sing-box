package clashapi

import (
	"bytes"
	"net/http"
	"time"

	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
	"github.com/sagernet/websocket"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// API created by Clash.Meta

func (s *Server) setupMetaAPI(r chi.Router) {
	r.Get("/memory", memory(s.trafficManager))
	r.Mount("/group", groupRouter(s))
}

type Memory struct {
	Inuse   uint64 `json:"inuse"`
	OSLimit uint64 `json:"oslimit"` // maybe we need it in the future
}

func memory(trafficManager *trafficontrol.Manager) func(w http.ResponseWriter, r *http.Request) {
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
