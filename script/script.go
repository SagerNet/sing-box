package script

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

func NewScript(ctx context.Context, logger logger.ContextLogger, options option.Script) (adapter.Script, error) {
	switch options.Type {
	case C.ScriptTypeSurgeGeneric:
		return NewSurgeGenericScript(ctx, logger, options)
	case C.ScriptTypeSurgeHTTPRequest:
		return NewSurgeHTTPRequestScript(ctx, logger, options)
	case C.ScriptTypeSurgeHTTPResponse:
		return NewSurgeHTTPResponseScript(ctx, logger, options)
	case C.ScriptTypeSurgeCron:
		return NewSurgeCronScript(ctx, logger, options)
	default:
		return nil, E.New("unknown script type: ", options.Type)
	}
}
