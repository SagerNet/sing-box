package deprecated

import (
	"context"

	"github.com/sagernet/sing/service"
)

type Manager interface {
	ReportDeprecated(note Note)
}

func Report(ctx context.Context, note Note) {
	manager := service.FromContext[Manager](ctx)
	if manager == nil {
		return
	}
	manager.ReportDeprecated(note)
}
