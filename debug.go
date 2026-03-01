package box

import (
	"runtime/debug"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func applyDebugOptions(options option.DebugOptions) error {
	applyDebugListenOption(options)
	if options.GCPercent != nil {
		debug.SetGCPercent(*options.GCPercent)
	}
	if options.MaxStack != nil {
		debug.SetMaxStack(*options.MaxStack)
	}
	if options.MaxThreads != nil {
		debug.SetMaxThreads(*options.MaxThreads)
	}
	if options.PanicOnFault != nil {
		debug.SetPanicOnFault(*options.PanicOnFault)
	}
	if options.TraceBack != "" {
		debug.SetTraceback(options.TraceBack)
	}
	if options.MemoryLimit.Value() != 0 {
		debug.SetMemoryLimit(int64(float64(options.MemoryLimit.Value()) / 1.5))
	}
	if options.OOMKiller != nil {
		return E.New("legacy oom_killer in debug options is removed, use oom-killer service instead")
	}
	return nil
}
