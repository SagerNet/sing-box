package ssmapi

import (
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func (s *Server) setupRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Get("/", s.getServerInfo)

		r.Get("/nodes", s.getNodes)
		r.Post("/nodes", s.addNode)
		r.Get("/nodes/{id}", s.getNode)
		r.Put("/nodes/{id}", s.updateNode)
		r.Delete("/nodes/{id}", s.deleteNode)

		r.Get("/users", s.listUser)
		r.Post("/users", s.addUser)
		r.Get("/users/{username}", s.getUser)
		r.Put("/users/{username}", s.updateUser)
		r.Delete("/users/{username}", s.deleteUser)

		r.Get("/stats", s.getStats)
	})
}

func (s *Server) getServerInfo(writer http.ResponseWriter, request *http.Request) {
	render.JSON(writer, request, render.M{
		"server":            "sing-box",
		"apiVersion":        "v1",
		"_sing_box_version": C.Version,
	})
}

func (s *Server) getNodes(writer http.ResponseWriter, request *http.Request) {
	var response struct {
		Protocols   []string                `json:"protocols"`
		Shadowsocks []ShadowsocksNodeObject `json:"shadowsocks,omitempty"`
	}
	for _, node := range s.nodes {
		if !common.Contains(response.Protocols, node.Protocol()) {
			response.Protocols = append(response.Protocols, node.Protocol())
		}
		switch node.Protocol() {
		case C.TypeShadowsocks:
			response.Shadowsocks = append(response.Shadowsocks, node.Shadowsocks())
		}
	}
	render.JSON(writer, request, &response)
}

func (s *Server) addNode(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusNotImplemented)
}

func (s *Server) getNode(writer http.ResponseWriter, request *http.Request) {
	nodeID := chi.URLParam(request, "id")
	if nodeID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	for _, node := range s.nodes {
		if nodeID == node.ID() {
			render.JSON(writer, request, render.M{
				node.Protocol(): node.Object(),
			})
			return
		}
	}
}

func (s *Server) updateNode(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusNotImplemented)
}

func (s *Server) deleteNode(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusNotImplemented)
}

type SSMUserObject struct {
	UserName      string `json:"username"`
	Password      string `json:"uPSK,omitempty"`
	DownlinkBytes int64  `json:"downlinkBytes"`
	UplinkBytes   int64  `json:"uplinkBytes"`

	DownlinkPackets int64 `json:"downlinkPackets"`
	UplinkPackets   int64 `json:"uplinkPackets"`
	TCPSessions     int64 `json:"tcpSessions"`
	UDPSessions     int64 `json:"udpSessions"`
}

func (s *Server) listUser(writer http.ResponseWriter, request *http.Request) {
	render.JSON(writer, request, render.M{
		"users": s.userManager.List(),
	})
}

func (s *Server) addUser(writer http.ResponseWriter, request *http.Request) {
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
	err = s.userManager.Add(addRequest.UserName, addRequest.Password)
	if err != nil {
		render.Status(request, http.StatusBadRequest)
		render.PlainText(writer, request, err.Error())
		return
	}
	writer.WriteHeader(http.StatusCreated)
}

func (s *Server) getUser(writer http.ResponseWriter, request *http.Request) {
	userName := chi.URLParam(request, "username")
	if userName == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	uPSK, loaded := s.userManager.Get(userName)
	if !loaded {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	user := SSMUserObject{
		UserName: userName,
		Password: uPSK,
	}
	s.trafficManager.ReadUser(&user)
	render.JSON(writer, request, user)
}

func (s *Server) updateUser(writer http.ResponseWriter, request *http.Request) {
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
	_, loaded := s.userManager.Get(userName)
	if !loaded {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	err = s.userManager.Update(userName, updateRequest.Password)
	if err != nil {
		render.Status(request, http.StatusBadRequest)
		render.PlainText(writer, request, err.Error())
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteUser(writer http.ResponseWriter, request *http.Request) {
	userName := chi.URLParam(request, "username")
	if userName == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	_, loaded := s.userManager.Get(userName)
	if !loaded {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	err := s.userManager.Delete(userName)
	if err != nil {
		render.Status(request, http.StatusBadRequest)
		render.PlainText(writer, request, err.Error())
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (s *Server) getStats(writer http.ResponseWriter, request *http.Request) {
	requireClear := chi.URLParam(request, "clear") == "true"

	users := s.userManager.List()
	s.trafficManager.ReadUsers(users)
	for i := range users {
		users[i].Password = ""
	}
	uplinkBytes, downlinkBytes, uplinkPackets, downlinkPackets, tcpSessions, udpSessions := s.trafficManager.ReadGlobal()

	if requireClear {
		s.trafficManager.Clear()
	}

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
