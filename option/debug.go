package option

import (
	"encoding/json"

	"github.com/dustin/go-humanize"
)

type DebugOptions struct {
	Listen       string      `json:"listen,omitempty"`
	GCPercent    *int        `json:"gc_percent,omitempty"`
	MaxStack     *int        `json:"max_stack,omitempty"`
	MaxThreads   *int        `json:"max_threads,omitempty"`
	PanicOnFault *bool       `json:"panic_on_fault,omitempty"`
	TraceBack    string      `json:"trace_back,omitempty"`
	MemoryLimit  BytesLength `json:"memory_limit,omitempty"`
	OOMKiller    *bool       `json:"oom_killer,omitempty"`
}

type BytesLength int64

func (l BytesLength) MarshalJSON() ([]byte, error) {
	return json.Marshal(humanize.IBytes(uint64(l)))
}

func (l *BytesLength) UnmarshalJSON(bytes []byte) error {
	var valueInteger int64
	err := json.Unmarshal(bytes, &valueInteger)
	if err == nil {
		*l = BytesLength(valueInteger)
		return nil
	}
	var valueString string
	err = json.Unmarshal(bytes, &valueString)
	if err != nil {
		return err
	}
	parsedValue, err := humanize.ParseBytes(valueString)
	if err != nil {
		return err
	}
	*l = BytesLength(parsedValue)
	return nil
}
