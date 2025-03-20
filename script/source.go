//go:build with_script

package script

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	"github.com/dop251/goja"
)

type Source interface {
	StartContext(ctx context.Context, startContext *adapter.HTTPStartContext) error
	PostStart() error
	Program() *goja.Program
	Close() error
}

func NewSource(ctx context.Context, logger logger.Logger, options option.Script) (Source, error) {
	switch options.Source {
	case C.ScriptSourceTypeLocal:
		return NewLocalSource(ctx, logger, options)
	case C.ScriptSourceTypeRemote:
		return NewRemoteSource(ctx, logger, options)
	default:
		return nil, E.New("unknown source type: ", options.Source)
	}
}
