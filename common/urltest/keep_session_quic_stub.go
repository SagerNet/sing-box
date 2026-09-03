//go:build !with_quic

package urltest

import "context"

func contextWithQUICKeepSession(ctx context.Context) context.Context {
	return ctx
}
