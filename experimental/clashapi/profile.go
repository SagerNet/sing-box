package clashapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func profileRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/tracing", subscribeTracing)
	return r
}

func subscribeTracing(w http.ResponseWriter, r *http.Request) {
	// if !profile.Tracing.Load() {
	render.Status(r, http.StatusNotFound)
	render.JSON(w, r, ErrNotFound)
	//return
	//}

	/*wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	ch := make(chan map[string]any, 1024)
	sub := event.Subscribe()
	defer event.UnSubscribe(sub)
	buf := &bytes.Buffer{}

	go func() {
		for elm := range sub {
			select {
			case ch <- elm:
			default:
			}
		}
		close(ch)
	}()

	for elm := range ch {
		buf.Reset()
		if err := json.NewEncoder(buf).Encode(elm); err != nil {
			break
		}

		if err := wsConn.WriteMessage(websocket.TextMessage, buf.Bytes()); err != nil {
			break
		}
	}*/
}
