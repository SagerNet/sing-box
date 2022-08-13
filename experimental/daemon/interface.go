package daemon

import "github.com/kardianos/service"

type Interface struct {
	server *Server
}

func NewInterface(options Options) *Interface {
	return &Interface{NewServer(options)}
}

func (d *Interface) Start(_ service.Service) error {
	return d.server.Start()
}

func (d *Interface) Stop(_ service.Service) error {
	d.server.Close()
	return nil
}
