package experimental

import (
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

type SSMServerConstructor = func(router adapter.Router, logger log.Logger, options option.SSMAPIOptions) (adapter.SSMServer, error)

var ssmServerConstructor SSMServerConstructor

func RegisterSSMServerConstructor(constructor SSMServerConstructor) {
	ssmServerConstructor = constructor
}

func NewSSMServer(router adapter.Router, logger log.Logger, options option.SSMAPIOptions) (adapter.SSMServer, error) {
	if ssmServerConstructor == nil {
		return nil, os.ErrInvalid
	}
	return ssmServerConstructor(router, logger, options)
}
