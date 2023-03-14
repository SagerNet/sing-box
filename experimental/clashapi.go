package experimental

import (
	"os"

	"github.com/jobberrt/sing-box/adapter"
	"github.com/jobberrt/sing-box/log"
	"github.com/jobberrt/sing-box/option"
)

type ClashServerConstructor = func(router adapter.Router, logFactory log.ObservableFactory, options option.ClashAPIOptions) (adapter.ClashServer, error)

var clashServerConstructor ClashServerConstructor

func RegisterClashServerConstructor(constructor ClashServerConstructor) {
	clashServerConstructor = constructor
}

func NewClashServer(router adapter.Router, logFactory log.ObservableFactory, options option.ClashAPIOptions) (adapter.ClashServer, error) {
	if clashServerConstructor == nil {
		return nil, os.ErrInvalid
	}
	return clashServerConstructor(router, logFactory, options)
}
