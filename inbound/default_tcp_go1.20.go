//go:build go1.20

package inbound

import (
	"context"
	"net"

	"github.com/metacubex/tfo-go"
)

const go120Available = true

func listenTFO(listenConfig net.ListenConfig, ctx context.Context, network string, address string) (net.Listener, error) {
	var tfoConfig tfo.ListenConfig
	tfoConfig.ListenConfig = listenConfig
	return tfoConfig.Listen(ctx, network, address)
}
