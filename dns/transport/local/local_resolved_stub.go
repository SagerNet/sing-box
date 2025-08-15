//go:build !linux

package local

import (
	"context"
	"os"

	"github.com/sagernet/sing/common/logger"
)

func NewResolvedResolver(ctx context.Context, logger logger.ContextLogger) (ResolvedResolver, error) {
	return nil, os.ErrInvalid
}
