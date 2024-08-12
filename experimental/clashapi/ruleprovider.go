package clashapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
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

func ruleSetInfo(ruleSet adapter.RuleSet) *badjson.JSONObject {
	var info badjson.JSONObject
	info.Put("name", ruleSet.Name())
	info.Put("type", "Rule")
	info.Put("vehicleType", strings.ToUpper(ruleSet.Type()))
	info.Put("behavior", strings.ToUpper(ruleSet.Format()))
	info.Put("ruleCount", ruleSet.RuleCount())
	info.Put("updatedAt", ruleSet.UpdatedTime().Format("2006-01-02T15:04:05.999999999-07:00"))
	return &info
}

func getRuleProviders(router adapter.Router) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		providerMap := render.M{}
		for i, ruleSet := range router.RuleSets() {
			var tag string
			if ruleSet.Name() == "" {
				tag = F.ToString(i)
			} else {
				tag = ruleSet.Name()
			}
			providerMap[tag] = ruleSetInfo(ruleSet)
		}
		render.JSON(w, r, render.M{
			"providers": providerMap,
		})
	}
}

func getRuleProvider(w http.ResponseWriter, r *http.Request) {
	ruleSet := r.Context().Value(CtxKeyProvider).(adapter.RuleSet)
	response, err := ruleSetInfo(ruleSet).MarshalJSON()
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, newError(err.Error()))
		return
	}
	w.Write(response)
}

func updateRuleProvider(w http.ResponseWriter, r *http.Request) {
	ruleSet := r.Context().Value(CtxKeyProvider).(adapter.RuleSet)
	err := ruleSet.Update(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, newError(err.Error()))
		return
	}
	render.NoContent(w, r)
}

func findRuleProviderByName(router adapter.Router) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := r.Context().Value(CtxKeyProviderName).(string)
			provider, exist := router.RuleSet(name)
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
