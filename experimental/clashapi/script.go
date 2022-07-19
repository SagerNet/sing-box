package clashapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func scriptRouter() http.Handler {
	r := chi.NewRouter()
	r.Post("/", testScript)
	r.Patch("/", patchScript)
	return r
}

/*type TestScriptRequest struct {
	Script   *string    `json:"script"`
	Metadata C.Metadata `json:"metadata"`
}*/

func testScript(w http.ResponseWriter, r *http.Request) {
	/*	req := TestScriptRequest{}
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, ErrBadRequest)
			return
		}

		fn := tunnel.ScriptFn()
		if req.Script == nil && fn == nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError("should send `script`"))
			return
		}

		if !req.Metadata.Valid() {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError("metadata not valid"))
			return
		}

		if req.Script != nil {
			var err error
			fn, err = script.ParseScript(*req.Script)
			if err != nil {
				render.Status(r, http.StatusBadRequest)
				render.JSON(w, r, newError(err.Error()))
				return
			}
		}

		ctx, _ := script.MakeContext(tunnel.ProxyProviders(), tunnel.RuleProviders())

		thread := &starlark.Thread{}
		ret, err := starlark.Call(thread, fn, starlark.Tuple{ctx, script.MakeMetadata(&req.Metadata)}, nil)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError(err.Error()))
			return
		}

		elm, ok := ret.(starlark.String)
		if !ok {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, "script fn must return a string")
			return
		}

		render.JSON(w, r, render.M{
			"result": string(elm),
		})*/
	render.Status(r, http.StatusBadRequest)
	render.JSON(w, r, newError("not implemented"))
}

type PatchScriptRequest struct {
	Script string `json:"script"`
}

func patchScript(w http.ResponseWriter, r *http.Request) {
	/*req := PatchScriptRequest{}
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrBadRequest)
		return
	}

	fn, err := script.ParseScript(req.Script)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, newError(err.Error()))
		return
	}

	tunnel.UpdateScript(fn)*/
	render.NoContent(w, r)
}
