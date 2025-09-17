//go:build linux && go1.25 && !badlinkname

package ktls

import (
	"context"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	aTLS "github.com/sagernet/sing/common/tls"
)

func NewConn(ctx context.Context, logger logger.ContextLogger, conn aTLS.Conn, txOffload, rxOffload bool) (aTLS.Conn, error) {
	return nil, E.New("kTLS requires build flags `badlinkname` and `-ldflags=-checklinkname=0`, please recompile your binary")
}
