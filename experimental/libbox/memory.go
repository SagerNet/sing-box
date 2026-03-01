package libbox

import (
	"math"
	runtimeDebug "runtime/debug"

	C "github.com/sagernet/sing-box/constant"
)

var memoryLimitEnabled bool

func SetMemoryLimit(enabled bool) {
	memoryLimitEnabled = enabled
	const memoryLimitGo = 45 * 1024 * 1024
	if enabled {
		runtimeDebug.SetGCPercent(10)
		if C.IsIos {
			runtimeDebug.SetMemoryLimit(memoryLimitGo)
		}
	} else {
		runtimeDebug.SetGCPercent(100)
		if C.IsIos {
			runtimeDebug.SetMemoryLimit(math.MaxInt64)
		}
	}
}
