//go:build darwin && cgo

package oomkiller

/*
#include <dispatch/dispatch.h>

static dispatch_source_t memoryPressureSource;

extern void goMemoryPressureCallback(unsigned long status);

static void startMemoryPressureMonitor() {
	memoryPressureSource = dispatch_source_create(
		DISPATCH_SOURCE_TYPE_MEMORYPRESSURE,
		0,
		DISPATCH_MEMORYPRESSURE_WARN | DISPATCH_MEMORYPRESSURE_CRITICAL,
		dispatch_get_global_queue(QOS_CLASS_DEFAULT, 0)
	);
	dispatch_source_set_event_handler(memoryPressureSource, ^{
		unsigned long status = dispatch_source_get_data(memoryPressureSource);
		goMemoryPressureCallback(status);
	});
	dispatch_activate(memoryPressureSource);
}

static void stopMemoryPressureMonitor() {
	if (memoryPressureSource) {
		dispatch_source_cancel(memoryPressureSource);
		memoryPressureSource = NULL;
	}
}
*/
import "C"

import (
	"context"
	runtimeDebug "runtime/debug"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	boxConstant "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/memory"
	"github.com/sagernet/sing/service"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.OOMKillerServiceOptions](registry, boxConstant.TypeOOMKiller, NewService)
}

var (
	globalAccess   sync.Mutex
	globalServices []*Service
)

type Service struct {
	boxService.Adapter
	logger        log.ContextLogger
	router        adapter.Router
	memoryLimit   uint64
	hasTimerMode  bool
	useAvailable  bool
	timerConfig   timerConfig
	adaptiveTimer *adaptiveTimer
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.OOMKillerServiceOptions) (adapter.Service, error) {
	s := &Service{
		Adapter: boxService.NewAdapter(boxConstant.TypeOOMKiller, tag),
		logger:  logger,
		router:  service.FromContext[adapter.Router](ctx),
	}

	if options.MemoryLimit != nil {
		s.memoryLimit = options.MemoryLimit.Value()
		if s.memoryLimit > 0 {
			s.hasTimerMode = true
		}
	}

	config, err := buildTimerConfig(options, s.memoryLimit, s.useAvailable)
	if err != nil {
		return nil, err
	}
	s.timerConfig = config

	return s, nil
}

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}

	if s.hasTimerMode {
		s.adaptiveTimer = newAdaptiveTimer(s.logger, s.router, s.timerConfig)
		if s.memoryLimit > 0 {
			s.logger.Info("started memory monitor with limit: ", s.memoryLimit/(1024*1024), " MiB")
		} else {
			s.logger.Info("started memory monitor with available memory detection")
		}
	} else {
		s.logger.Info("started memory pressure monitor")
	}

	globalAccess.Lock()
	isFirst := len(globalServices) == 0
	globalServices = append(globalServices, s)
	globalAccess.Unlock()

	if isFirst {
		C.startMemoryPressureMonitor()
	}
	return nil
}

func (s *Service) Close() error {
	if s.adaptiveTimer != nil {
		s.adaptiveTimer.stop()
	}
	globalAccess.Lock()
	for i, svc := range globalServices {
		if svc == s {
			globalServices = append(globalServices[:i], globalServices[i+1:]...)
			break
		}
	}
	isLast := len(globalServices) == 0
	globalAccess.Unlock()
	if isLast {
		C.stopMemoryPressureMonitor()
	}
	return nil
}

//export goMemoryPressureCallback
func goMemoryPressureCallback(status C.ulong) {
	globalAccess.Lock()
	services := make([]*Service, len(globalServices))
	copy(services, globalServices)
	globalAccess.Unlock()
	if len(services) == 0 {
		return
	}
	criticalFlag := C.ulong(C.DISPATCH_MEMORYPRESSURE_CRITICAL)
	warnFlag := C.ulong(C.DISPATCH_MEMORYPRESSURE_WARN)
	isCritical := status&criticalFlag != 0
	isWarning := status&warnFlag != 0
	var level string
	switch {
	case isCritical:
		level = "critical"
	case isWarning:
		level = "warning"
	default:
		level = "normal"
	}
	var freeOSMemory bool
	for _, s := range services {
		usage := memory.Total()
		if s.hasTimerMode {
			if isCritical {
				s.logger.Warn("memory pressure: ", level, ", usage: ", usage/(1024*1024), " MiB")
				if s.adaptiveTimer != nil {
					s.adaptiveTimer.startNow()
				}
			} else if isWarning {
				s.logger.Warn("memory pressure: ", level, ", usage: ", usage/(1024*1024), " MiB")
			} else {
				s.logger.Debug("memory pressure: ", level, ", usage: ", usage/(1024*1024), " MiB")
				if s.adaptiveTimer != nil {
					s.adaptiveTimer.stop()
				}
			}
		} else {
			if isCritical {
				s.logger.Error("memory pressure: ", level, ", usage: ", usage/(1024*1024), " MiB, resetting network")
				s.router.ResetNetwork()
				freeOSMemory = true
			} else if isWarning {
				s.logger.Warn("memory pressure: ", level, ", usage: ", usage/(1024*1024), " MiB")
			} else {
				s.logger.Debug("memory pressure: ", level, ", usage: ", usage/(1024*1024), " MiB")
			}
		}
	}
	if freeOSMemory {
		runtimeDebug.FreeOSMemory()
	}
}
