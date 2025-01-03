package libbox

import (
	"math"
	runtimeDebug "runtime/debug"

	"github.com/sagernet/sing-box/common/conntrack"
)

var tracker *conntrack.DefaultTracker

func SetMemoryLimit(enabled bool) {
	if tracker != nil {
		tracker.Close()
	}
	const memoryLimit = 45 * 1024 * 1024
	const memoryLimitGo = memoryLimit / 1.5
	if enabled {
		runtimeDebug.SetGCPercent(10)
		runtimeDebug.SetMemoryLimit(memoryLimitGo)
		tracker = conntrack.NewDefaultTracker(true, memoryLimit)
	} else {
		runtimeDebug.SetGCPercent(100)
		runtimeDebug.SetMemoryLimit(math.MaxInt64)
		tracker = conntrack.NewDefaultTracker(false, 0)
	}
}
