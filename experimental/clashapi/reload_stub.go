//go:build ios

package clashapi

import (
	"net/http"

	"github.com/go-chi/render"
)

var ErrOSNotSupported = &HTTPError{
	Message: "OS not supported",
}

func reload(server *Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, ErrOSNotSupported)
	}
}
