package clashapi

import (
	"bytes"
	"net/http"
	"strconv"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
	"github.com/sagernet/websocket"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func connectionRouter(router adapter.Router, trafficManager *trafficontrol.Manager) http.Handler {
	r := chi.NewRouter()
	r.Get("/", getConnections(trafficManager))
	r.Delete("/", closeAllConnections(router, trafficManager))
	r.Delete("/{id}", closeConnection(trafficManager))
	return r
}

func getConnections(trafficManager *trafficontrol.Manager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if !websocket.IsWebSocketUpgrade(r) {
			snapshot := trafficManager.Snapshot()
			render.JSON(w, r, snapshot)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		intervalStr := r.URL.Query().Get("interval")
		interval := 1000
		if intervalStr != "" {
			t, err := strconv.Atoi(intervalStr)
			if err != nil {
				render.Status(r, http.StatusBadRequest)
				render.JSON(w, r, ErrBadRequest)
				return
			}

			interval = t
		}

		buf := &bytes.Buffer{}
		sendSnapshot := func() error {
			buf.Reset()
			snapshot := trafficManager.Snapshot()
			if err := json.NewEncoder(buf).Encode(snapshot); err != nil {
				return err
			}
			return conn.WriteMessage(websocket.TextMessage, buf.Bytes())
		}

		if err = sendSnapshot(); err != nil {
			return
		}

		tick := time.NewTicker(time.Millisecond * time.Duration(interval))
		defer tick.Stop()
		for range tick.C {
			if err = sendSnapshot(); err != nil {
				break
			}
		}
	}
}

func closeConnection(trafficManager *trafficontrol.Manager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		snapshot := trafficManager.Snapshot()
		for _, c := range snapshot.Connections {
			if id == c.ID() {
				c.Close()
				break
			}
		}
		render.NoContent(w, r)
	}
}

func closeAllConnections(router adapter.Router, trafficManager *trafficontrol.Manager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot := trafficManager.Snapshot()
		for _, c := range snapshot.Connections {
			c.Close()
		}
		router.ResetNetwork()
		render.NoContent(w, r)
	}
}
