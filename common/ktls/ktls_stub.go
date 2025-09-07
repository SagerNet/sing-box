//go:build !linux || !go1.25 || without_badtls

package ktls

import (
	"context"
	"os"

	"github.com/sagernet/sing/common/logger"
	aTLS "github.com/sagernet/sing/common/tls"
)

func NewConn(ctx context.Context, logger logger.ContextLogger, conn aTLS.Conn, txOffload, rxOffload bool) (aTLS.Conn, error) {
	return nil, os.ErrInvalid
}
