//go:build with_quic

package urltest

import (
	"context"

	"github.com/sagernet/sing-quic"
)

func contextWithQUICKeepSession(ctx context.Context) context.Context {
	return qtls.ContextWithKeepSession(ctx)
}
