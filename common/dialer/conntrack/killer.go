package conntrack

import (
	"runtime"
	runtimeDebug "runtime/debug"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

var (
	KillerEnabled   bool
	MemoryLimit     int64
	killerLastCheck time.Time
)

func killerCheck() error {
	if !KillerEnabled {
		return nil
	}
	nowTime := time.Now()
	if nowTime.Sub(killerLastCheck) < 3*time.Second {
		return nil
	}
	killerLastCheck = nowTime
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	inuseMemory := int64(memStats.StackInuse + memStats.HeapInuse + memStats.HeapIdle - memStats.HeapReleased)
	if inuseMemory > MemoryLimit {
		Close()
		go func() {
			time.Sleep(time.Second)
			runtimeDebug.FreeOSMemory()
		}()
		return E.New("out of memory")
	}
	return nil
}
