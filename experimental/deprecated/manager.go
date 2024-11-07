package deprecated

import (
	"context"
	"runtime/debug"

	"github.com/sagernet/sing/service"
)

type Manager interface {
	ReportDeprecated(feature Note)
}

func Report(ctx context.Context, feature Note) {
	manager := service.FromContext[Manager](ctx)
	if manager == nil {
		debug.PrintStack()
		return
	}
	manager.ReportDeprecated(feature)
}
