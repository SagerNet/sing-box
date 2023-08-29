//go:build !with_ech

package tls

import (
	"context"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

var errECHNotIncluded = E.New(`ECH is not included in this build, rebuild with -tags with_ech`)

func NewECHServer(ctx context.Context, logger log.Logger, options option.InboundTLSOptions) (ServerConfig, error) {
	return nil, errECHNotIncluded
}

func NewECHClient(ctx context.Context, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
	return nil, errECHNotIncluded
}

func ECHKeygenDefault(host string, pqSignatureSchemesEnabled bool) (configPem string, keyPem string, err error) {
	return "", "", errECHNotIncluded
}
