//go:build !go1.20

package inbound

import (
	"context"
	"net"
	"os"
)

const go120Available = false

func listenTFO(listenConfig net.ListenConfig, ctx context.Context, network string, address string) (net.Listener, error) {
	return nil, os.ErrInvalid
}
