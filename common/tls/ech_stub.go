//go:build !with_ech

package tls

import (
	"github.com/jobberrt/sing-box/adapter"
	"github.com/jobberrt/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewECHClient(router adapter.Router, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
	return nil, E.New(`ECH is not included in this build, rebuild with -tags with_ech`)
}
