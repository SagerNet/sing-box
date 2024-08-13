package clashapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json/badjson"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func proxyProviderRouter(server *Server, router adapter.Router) http.Handler {
	r := chi.NewRouter()
	r.Get("/", getProviders(server, router))

	r.Route("/{name}", func(r chi.Router) {
		r.Use(parseProviderName, findProviderByName(router))
		r.Get("/", getProvider(server))
		r.Put("/", updateProvider(server, router))
		r.Get("/healthcheck", healthCheckProvider(server))
	})
	return r
}

func providerInfo(server *Server, provider adapter.OutboundProvider) *render.M {
	return &render.M{
		"name":             provider.Tag(),
		"type":             "Proxy",
		"vehicleType":      strings.ToUpper(provider.Type()),
		"subscriptionInfo": provider.SubInfo(),
		"updatedAt":        provider.UpdateTime().Format("2006-01-02T15:04:05.999999999-07:00"),
		"proxies": common.Map(provider.Outbounds(), func(it adapter.Outbound) *badjson.JSONObject {
			return proxyInfo(server, it)
		}),
	}
}

func getProviders(server *Server, router adapter.Router) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		providerMap := make(render.M)
		for i, provider := range router.OutboundProviders() {
			var tag string
			if provider.Tag() == "" {
				tag = F.ToString(i)
			} else {
				tag = provider.Tag()
			}
			providerMap[tag] = providerInfo(server, provider)
		}
		render.JSON(w, r, render.M{
			"providers": providerMap,
		})
	}
}

func getProvider(server *Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := r.Context().Value(CtxKeyProvider).(adapter.OutboundProvider)
		render.JSON(w, r, providerInfo(server, provider))
	}
}

func updateProvider(server *Server, router adapter.Router) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := r.Context().Value(CtxKeyProvider).(adapter.OutboundProvider)
		err := provider.UpdateProvider(server.ctx, router)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, err)
			return
		}
		render.NoContent(w, r)
	}
}

func healthCheckProvider(server *Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := r.Context().Value(CtxKeyProvider).(adapter.OutboundProvider)

		query := r.URL.Query()
		link := query.Get("url")
		timeout := int64(5000)

		ctx, cancel := context.WithTimeout(r.Context(), time.Millisecond*time.Duration(timeout))
		defer cancel()

		render.JSON(w, r, provider.Healthcheck(ctx, link, true))
	}
}

func parseProviderName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := getEscapeParam(r, "name")
		ctx := context.WithValue(r.Context(), CtxKeyProviderName, name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func findProviderByName(router adapter.Router) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := r.Context().Value(CtxKeyProviderName).(string)
			provider, exist := router.OutboundProvider(name)
			if !exist {
				render.Status(r, http.StatusNotFound)
				render.JSON(w, r, ErrNotFound)
				return
			}
			ctx := context.WithValue(r.Context(), CtxKeyProvider, provider)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
