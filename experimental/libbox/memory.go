package libbox

import (
	"math"
	runtimeDebug "runtime/debug"

	"github.com/sagernet/sing-box/common/dialer/conntrack"
)

func SetMemoryLimit(enabled bool) {
	const memoryLimit = 30 * 1024 * 1024
	if enabled {
		runtimeDebug.SetGCPercent(10)
		runtimeDebug.SetMemoryLimit(memoryLimit)
		conntrack.KillerEnabled = true
		conntrack.MemoryLimit = memoryLimit
	} else {
		runtimeDebug.SetGCPercent(100)
		runtimeDebug.SetMemoryLimit(math.MaxInt64)
		conntrack.KillerEnabled = false
	}
}
