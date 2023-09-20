package conntrack

import (
	runtimeDebug "runtime/debug"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/memory"
)

var (
	KillerEnabled   bool
	MemoryLimit     uint64
	killerLastCheck time.Time
)

func KillerCheck() error {
	if !KillerEnabled {
		return nil
	}
	nowTime := time.Now()
	if nowTime.Sub(killerLastCheck) < 3*time.Second {
		return nil
	}
	killerLastCheck = nowTime
	if memory.Total() > MemoryLimit {
		Close()
		go func() {
			time.Sleep(time.Second)
			runtimeDebug.FreeOSMemory()
		}()
		return E.New("out of memory")
	}
	return nil
}
