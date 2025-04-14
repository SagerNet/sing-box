package ssmapi

import (
	"net/http"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/logger"
	sHTTP "github.com/sagernet/sing/protocol/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type APIServer struct {
	logger  logger.Logger
	traffic *TrafficManager
	user    *UserManager
}

func NewAPIServer(logger logger.Logger, traffic *TrafficManager, user *UserManager) *APIServer {
	return &APIServer{
		logger:  logger,
		traffic: traffic,
		user:    user,
	}
}

func (s *APIServer) Route(r chi.Router) {
	r.Route("/server/v1", func(r chi.Router) {
		r.Use(func(handler http.Handler) http.Handler {
			return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				s.logger.Debug(request.Method, " ", request.RequestURI, " ", sHTTP.SourceAddress(request))
				handler.ServeHTTP(writer, request)
			})
		})
		r.Get("/", s.getServerInfo)
		r.Get("/users", s.listUser)
		r.Post("/users", s.addUser)
		r.Get("/users/{username}", s.getUser)
		r.Put("/users/{username}", s.updateUser)
		r.Delete("/users/{username}", s.deleteUser)
		r.Get("/stats", s.getStats)
	})
}

func (s *APIServer) getServerInfo(writer http.ResponseWriter, request *http.Request) {
	render.JSON(writer, request, render.M{
		"server":     "sing-box " + C.Version,
		"apiVersion": "v1",
	})
}

type UserObject struct {
	UserName        string `json:"username"`
	Password        string `json:"uPSK,omitempty"`
	DownlinkBytes   int64  `json:"downlinkBytes"`
	UplinkBytes     int64  `json:"uplinkBytes"`
	DownlinkPackets int64  `json:"downlinkPackets"`
	UplinkPackets   int64  `json:"uplinkPackets"`
	TCPSessions     int64  `json:"tcpSessions"`
	UDPSessions     int64  `json:"udpSessions"`
}

func (s *APIServer) listUser(writer http.ResponseWriter, request *http.Request) {
	render.JSON(writer, request, render.M{
		"users": s.user.List(),
	})
}

func (s *APIServer) addUser(writer http.ResponseWriter, request *http.Request) {
	var addRequest struct {
		UserName string `json:"username"`
		Password string `json:"uPSK"`
	}
	err := render.DecodeJSON(request.Body, &addRequest)
	if err != nil {
		render.Status(request, http.StatusBadRequest)
		render.PlainText(writer, request, err.Error())
		return
	}
	err = s.user.Add(addRequest.UserName, addRequest.Password)
	if err != nil {
		render.Status(request, http.StatusBadRequest)
		render.PlainText(writer, request, err.Error())
		return
	}
	writer.WriteHeader(http.StatusCreated)
}

func (s *APIServer) getUser(writer http.ResponseWriter, request *http.Request) {
	userName := chi.URLParam(request, "username")
	if userName == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	uPSK, loaded := s.user.Get(userName)
	if !loaded {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	user := UserObject{
		UserName: userName,
		Password: uPSK,
	}
	s.traffic.ReadUser(&user)
	render.JSON(writer, request, user)
}

func (s *APIServer) updateUser(writer http.ResponseWriter, request *http.Request) {
	userName := chi.URLParam(request, "username")
	if userName == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	var updateRequest struct {
		Password string `json:"uPSK"`
	}
	err := render.DecodeJSON(request.Body, &updateRequest)
	if err != nil {
		render.Status(request, http.StatusBadRequest)
		render.PlainText(writer, request, err.Error())
		return
	}
	_, loaded := s.user.Get(userName)
	if !loaded {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	err = s.user.Update(userName, updateRequest.Password)
	if err != nil {
		render.Status(request, http.StatusBadRequest)
		render.PlainText(writer, request, err.Error())
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (s *APIServer) deleteUser(writer http.ResponseWriter, request *http.Request) {
	userName := chi.URLParam(request, "username")
	if userName == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	_, loaded := s.user.Get(userName)
	if !loaded {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	err := s.user.Delete(userName)
	if err != nil {
		render.Status(request, http.StatusBadRequest)
		render.PlainText(writer, request, err.Error())
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (s *APIServer) getStats(writer http.ResponseWriter, request *http.Request) {
	requireClear := request.URL.Query().Get("clear") == "true"

	users := s.user.List()
	s.traffic.ReadUsers(users, requireClear)
	for i := range users {
		users[i].Password = ""
	}
	uplinkBytes, downlinkBytes, uplinkPackets, downlinkPackets, tcpSessions, udpSessions := s.traffic.ReadGlobal(requireClear)

	render.JSON(writer, request, render.M{
		"uplinkBytes":     uplinkBytes,
		"downlinkBytes":   downlinkBytes,
		"uplinkPackets":   uplinkPackets,
		"downlinkPackets": downlinkPackets,
		"tcpSessions":     tcpSessions,
		"udpSessions":     udpSessions,
		"users":           users,
	})
}
