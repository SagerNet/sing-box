package clashapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func cacheRouter() http.Handler {
	r := chi.NewRouter()
	r.Post("/fakeip/flush", flushFakeip)
	return r
}

func flushFakeip(w http.ResponseWriter, r *http.Request) {
	/*if err := cachefile.Cache().FlushFakeip(); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, newError(err.Error()))
		return
	}*/
	render.NoContent(w, r)
}
