package inbound

import (
	"encoding/json"
	"io"
	"net/http"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func (h *ShadowsocksMulti) createHandler() http.Handler {
	router := chi.NewRouter()
	router.Get("/", h.handleHello)
	router.Put("/users", h.handleUpdateUsers)
	router.Get("/traffics", h.handleReadTraffics)
	return router
}

func (h *ShadowsocksMulti) handleHello(writer http.ResponseWriter, request *http.Request) {
	render.JSON(writer, request, render.M{
		"server":  "sing-box",
		"version": C.Version,
	})
}

func (h *ShadowsocksMulti) handleUpdateUsers(writer http.ResponseWriter, request *http.Request) {
	var users []option.ShadowsocksUser
	err := readRequest(request, &users)
	if err != nil {
		h.newError(E.Cause(err, "controller: update users: parse request"))
		writer.WriteHeader(http.StatusBadRequest)
		writer.Write([]byte(F.ToString(err)))
		return
	}
	users = append([]option.ShadowsocksUser{{
		Name:     "control",
		Password: h.users[0].Password,
	}}, users...)
	err = h.service.UpdateUsersWithPasswords(common.MapIndexed(users, func(index int, user option.ShadowsocksUser) int {
		return index
	}), common.Map(users, func(user option.ShadowsocksUser) string {
		return user.Password
	}))
	if err != nil {
		h.newError(E.Cause(err, "controller: update users"))
		writer.WriteHeader(http.StatusBadRequest)
		writer.Write([]byte(F.ToString(err)))
		return
	}
	h.users = users
	h.trafficManager.Reset()
	writer.WriteHeader(http.StatusNoContent)
	h.logger.Info("controller: updated ", len(users)-1, " users")
}

type ShadowsocksUserTraffic struct {
	Name     string `json:"name,omitempty"`
	Upload   uint64 `json:"upload,omitempty"`
	Download uint64 `json:"download,omitempty"`
}

func (h *ShadowsocksMulti) handleReadTraffics(writer http.ResponseWriter, request *http.Request) {
	h.logger.Debug("controller: traffics sent")
	trafficMap := h.trafficManager.ReadTraffics()
	if len(trafficMap) == 0 {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	traffics := make([]ShadowsocksUserTraffic, 0, len(trafficMap))
	for user, traffic := range trafficMap {
		traffics = append(traffics, ShadowsocksUserTraffic{
			Name:     h.users[user].Name,
			Upload:   traffic.Upload,
			Download: traffic.Download,
		})
	}
	render.JSON(writer, request, traffics)
}

func readRequest(request *http.Request, v any) error {
	defer request.Body.Close()
	content, err := io.ReadAll(request.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(content, v)
}
