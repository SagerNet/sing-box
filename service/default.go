package service

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
)

type myServiceAdapter struct {
	serviceType string
	router      adapter.Router
	logger      log.ContextLogger
	tag         string
}

func (a *myServiceAdapter) Type() string {
	return a.serviceType
}

func (a *myServiceAdapter) Tag() string {
	return a.tag
}
