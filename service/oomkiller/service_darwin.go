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
		DISPATCH_MEMORYPRESSURE_CRITICAL,
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
	runtimeDebug "runtime/debug"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/byteformats"
	E "github.com/sagernet/sing/common/exceptions"
)

var (
	globalAccess   sync.Mutex
	globalServices []*Service
)

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if s.timerConfig.policyMode == policyModeNetworkExtension {
		s.createTimer()
		globalAccess.Lock()
		isFirst := len(globalServices) == 0
		globalServices = append(globalServices, s)
		globalAccess.Unlock()
		if isFirst {
			C.startMemoryPressureMonitor()
		}
		return nil
	}
	if !s.timerConfig.policyMode.hasTimerMode() {
		return E.New("memory pressure monitoring is not available on this platform without memory_limit")
	}
	s.startTimer()
	return nil
}

func (s *Service) Close() error {
	s.stopTimer()
	if s.timerConfig.policyMode == policyModeNetworkExtension {
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
		s.discardOOMDraft()
	}
	return nil
}

//export goMemoryPressureCallback
func goMemoryPressureCallback(status C.ulong) {
	runtimeDebug.FreeOSMemory()
	globalAccess.Lock()
	services := make([]*Service, len(globalServices))
	copy(services, globalServices)
	globalAccess.Unlock()
	if len(services) == 0 {
		return
	}
	sample := readMemorySample(policyModeNetworkExtension)
	for _, s := range services {
		s.logger.Warn("memory pressure: critical, usage: ", byteformats.FormatMemoryBytes(sample.usage))
		s.writeOOMDraft(sample.usage)
		s.adaptiveTimer.notifyPressure()
	}
}
