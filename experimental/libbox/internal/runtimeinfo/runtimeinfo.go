package runtimeinfo

import (
	"encoding/json"
	"os"
	"runtime/metrics"
)

type Report struct {
	Metrics    map[string]uint64 `json:"metrics"`
	Goroutines *GoroutineReport  `json:"goroutines,omitempty"`
	Pools      []PoolReport      `json:"bufferPools,omitempty"`
}

type GoroutineReport struct {
	Total          int              `json:"total"`
	StackBytes     uint64           `json:"stackBytes"`
	Dead           int              `json:"dead"`
	DeadStackBytes uint64           `json:"deadStackBytes"`
	ByStatus       map[string]int   `json:"byStatus"`
	ByFunction     []GoroutineGroup `json:"byFunction"`
}

type GoroutineGroup struct {
	Function      string `json:"function"`
	CreatedBy     string `json:"createdBy,omitempty"`
	Count         int    `json:"count"`
	StackBytes    uint64 `json:"stackBytes"`
	MaxStackBytes uint64 `json:"maxStackBytes"`
	MinStackBytes uint64 `json:"minStackBytes"`
}

type PoolReport struct {
	Size   int    `json:"size"`
	Cached int    `json:"cached"`
	Victim int    `json:"victim"`
	Bytes  uint64 `json:"bytes"`
}

func Collect() Report {
	return Report{
		Metrics:    collectMetrics(),
		Goroutines: collectGoroutines(),
		Pools:      collectBufferPools(),
	}
}

func WriteFile(path string) error {
	content, err := json.MarshalIndent(Collect(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o666)
}

func collectMetrics() map[string]uint64 {
	descriptions := metrics.All()
	samples := make([]metrics.Sample, 0, len(descriptions))
	for _, description := range descriptions {
		if description.Kind != metrics.KindUint64 {
			continue
		}
		samples = append(samples, metrics.Sample{Name: description.Name})
	}
	metrics.Read(samples)
	result := make(map[string]uint64, len(samples))
	for _, sample := range samples {
		if sample.Value.Kind() != metrics.KindUint64 {
			continue
		}
		result[sample.Name] = sample.Value.Uint64()
	}
	return result
}
