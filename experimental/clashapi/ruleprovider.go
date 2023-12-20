package clashapi

import (
	"context"
	"net/http"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/json/badjson"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func ruleProviderRouter(router adapter.Router) http.Handler {
	r := chi.NewRouter()
	r.Get("/", getRuleProviders(router))

	r.Route("/{name}", func(r chi.Router) {
		r.Use(parseProviderName, findRuleProviderByName(router))
		r.Get("/", getRuleProvider)
		r.Put("/", updateRuleProvider)
	})
	return r
}

func getRuleProviders(router adapter.Router) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruleSets := router.RuleSets()
		if len(ruleSets) == 0 {
			render.JSON(w, r, render.M{
				"providers": []string{},
			})
		}
		m := render.M{}
		for _, ruleSet := range ruleSets {
			m[ruleSet.Tag()] = ruleProviderInfo(ruleSet)
		}
		render.JSON(w, r, render.M{
			"providers": m,
		})
	}
}

func getRuleProvider(w http.ResponseWriter, r *http.Request) {
	ruleSet := r.Context().Value(CtxKeyProvider).(adapter.RuleSet)
	render.JSON(w, r, ruleProviderInfo(ruleSet))
}

func updateRuleProvider(w http.ResponseWriter, r *http.Request) {
	/*provider := r.Context().Value(CtxKeyProvider).(provider.RuleProvider)
	if err := provider.Update(); err != nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, newError(err.Error()))
		return
	}*/
	render.NoContent(w, r)
}

func findRuleProviderByName(router adapter.Router) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := r.Context().Value(CtxKeyProviderName).(string)
			ruleSet, exist := router.RuleSet(name)
			if !exist {
				render.Status(r, http.StatusNotFound)
				render.JSON(w, r, ErrNotFound)
				return
			}

			ctx := context.WithValue(r.Context(), CtxKeyProvider, ruleSet)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ruleProviderInfo(ruleSet adapter.RuleSet) *badjson.JSONObject {
	var info badjson.JSONObject
	info.Put("name", ruleSet.Tag())
	info.Put("type", "Rule")
	if ruleSet.Type() == "remote" {
		info.Put("vehicleType", "HTTP")
	} else {
		info.Put("vehicleType", "File")
	}
	metadata := ruleSet.Metadata()
	info.Put("format", metadata.Format)
	info.Put("behavior", "sing")
	info.Put("ruleCount", metadata.RuleNum)
	info.Put("updatedAt", metadata.LastUpdated)
	return &info
}
