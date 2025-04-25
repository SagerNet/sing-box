package clashapi

import (
	"net/http"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func upgradeRouter(server *Server) http.Handler {
	r := chi.NewRouter()
	r.Post("/ui", updateExternalUI(server))
	return r
}

func updateExternalUI(server *Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if server.externalUI == "" {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, newError("external UI not enabled"))
			return
		}
		server.logger.Info("upgrading external UI")
		err := server.downloadExternalUI()
		if err != nil {
			server.logger.Error(E.Cause(err, "upgrade external ui"))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, newError(err.Error()))
			return
		}
		server.logger.Info("updated external UI")
		render.JSON(w, r, render.M{"status": "ok"})
	}
}
