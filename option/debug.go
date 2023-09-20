package option

import (
	"encoding/json"

	"github.com/sagernet/sing-box/common/humanize"
)

type DebugOptions struct {
	Listen       string      `json:"listen,omitempty"`
	GCPercent    *int        `json:"gc_percent,omitempty"`
	MaxStack     *int        `json:"max_stack,omitempty"`
	MaxThreads   *int        `json:"max_threads,omitempty"`
	PanicOnFault *bool       `json:"panic_on_fault,omitempty"`
	TraceBack    string      `json:"trace_back,omitempty"`
	MemoryLimit  MemoryBytes `json:"memory_limit,omitempty"`
	OOMKiller    *bool       `json:"oom_killer,omitempty"`
}

type MemoryBytes uint64

func (l MemoryBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(humanize.MemoryBytes(uint64(l)))
}

func (l *MemoryBytes) UnmarshalJSON(bytes []byte) error {
	var valueInteger int64
	err := json.Unmarshal(bytes, &valueInteger)
	if err == nil {
		*l = MemoryBytes(valueInteger)
		return nil
	}
	var valueString string
	err = json.Unmarshal(bytes, &valueString)
	if err != nil {
		return err
	}
	parsedValue, err := humanize.ParseMemoryBytes(valueString)
	if err != nil {
		return err
	}
	*l = MemoryBytes(parsedValue)
	return nil
}
