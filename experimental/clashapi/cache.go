package clashapi

import (
	"net/http"

	"github.com/sagernet/sing-box/adapter"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func cacheRouter(router adapter.Router) http.Handler {
	r := chi.NewRouter()
	r.Post("/fakeip/flush", flushFakeip(router))
	return r
}

func flushFakeip(router adapter.Router) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if cacheFile := router.ClashServer().CacheFile(); cacheFile != nil {
			err := cacheFile.FakeIPReset()
			if err != nil {
				render.Status(r, http.StatusInternalServerError)
				render.JSON(w, r, newError(err.Error()))
				return
			}
		}
		render.NoContent(w, r)
	}
}
