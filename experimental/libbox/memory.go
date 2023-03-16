//go:build darwin

package libbox

import (
	runtimeDebug "runtime/debug"

	"github.com/sagernet/sing-box/common/dialer/conntrack"
)

const memoryLimit = 30 * 1024 * 1024

func SetMemoryLimit() {
	runtimeDebug.SetGCPercent(10)
	runtimeDebug.SetMemoryLimit(memoryLimit)
	conntrack.KillerEnabled = true
	conntrack.MemoryLimit = memoryLimit
}
