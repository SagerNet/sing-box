package clashapi

import (
	"context"
	"net/http"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func cacheRouter(ctx context.Context) http.Handler {
	r := chi.NewRouter()
	r.Post("/fakeip/flush", flushFakeip(ctx))
	r.Post("/dns/flush", flushDNS(ctx))
	return r
}

func flushFakeip(ctx context.Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cacheFile := service.FromContext[adapter.CacheFile](ctx)
		if cacheFile != nil {
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

func flushDNS(ctx context.Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		dnsRouter := service.FromContext[adapter.DNSRouter](ctx)
		if dnsRouter != nil {
			dnsRouter.ClearCache()
		}
		render.NoContent(w, r)
	}
}
