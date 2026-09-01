package daemon

import (
	"context"
	"time"
)

const defaultAttachedLogMaxLines = 3000

// StartOrReloadService and CloseService must not be called on an attached service.
func NewAttachedService(ctx context.Context) *StartedService {
	instance := attachInstance(ctx)
	s := NewStartedService(ServiceOptions{
		Context:     ctx,
		LogMaxLines: defaultAttachedLogMaxLines,
	})
	s.instance = instance
	s.serviceStatus = &ServiceStatus{Status: ServiceStatus_STARTED}
	s.startedAt = time.Now()
	instance.urlTestHistoryStorage.AddUpdateHook(s.urlTestSubscriber)
	if instance.clashMode != nil {
		instance.clashMode.AddUpdateHook(s.clashModeSubscriber)
	}
	instance.logFactory.AttachPlatformWriter(s)
	return s
}
