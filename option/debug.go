package option

import "github.com/sagernet/sing/common/byteformats"

type DebugOptions struct {
	Listen       string                   `json:"listen,omitempty"`
	GCPercent    *int                     `json:"gc_percent,omitempty"`
	MaxStack     *int                     `json:"max_stack,omitempty"`
	MaxThreads   *int                     `json:"max_threads,omitempty"`
	PanicOnFault *bool                    `json:"panic_on_fault,omitempty"`
	TraceBack    string                   `json:"trace_back,omitempty"`
	MemoryLimit  *byteformats.MemoryBytes `json:"memory_limit,omitempty"`
	OOMKiller    *bool                    `json:"oom_killer,omitempty"`
}
