//go:build darwin || linux || windows

package libbox

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sagernet/sing-box/experimental/libbox/internal/oomprofile"
	"github.com/sagernet/sing-box/service/oomkiller"
	"github.com/sagernet/sing/common/byteformats"
	"github.com/sagernet/sing/common/memory"
)

func init() {
	sOOMReporter = &oomReporter{}
}

var oomReportProfiles = []string{
	"allocs",
	"block",
	"goroutine",
	"heap",
	"mutex",
	"threadcreate",
}

type oomReportMetadata struct {
	reportMetadata
	RecordedAt      string `json:"recordedAt"`
	MemoryUsage     string `json:"memoryUsage"`
	AvailableMemory string `json:"availableMemory,omitempty"`
	// Heap
	HeapAlloc    string `json:"heapAlloc,omitempty"`
	HeapObjects  uint64 `json:"heapObjects,omitempty,string"`
	HeapInuse    string `json:"heapInuse,omitempty"`
	HeapIdle     string `json:"heapIdle,omitempty"`
	HeapReleased string `json:"heapReleased,omitempty"`
	HeapSys      string `json:"heapSys,omitempty"`
	// Stack
	StackInuse string `json:"stackInuse,omitempty"`
	StackSys   string `json:"stackSys,omitempty"`
	// Runtime metadata
	MSpanInuse  string `json:"mSpanInuse,omitempty"`
	MSpanSys    string `json:"mSpanSys,omitempty"`
	MCacheSys   string `json:"mCacheSys,omitempty"`
	BuckHashSys string `json:"buckHashSys,omitempty"`
	GCSys       string `json:"gcSys,omitempty"`
	OtherSys    string `json:"otherSys,omitempty"`
	Sys         string `json:"sys,omitempty"`
	// GC & runtime
	TotalAlloc   string `json:"totalAlloc,omitempty"`
	NumGC        uint32 `json:"numGC,omitempty,string"`
	NumGoroutine int    `json:"numGoroutine,omitempty,string"`
	NextGC       string `json:"nextGC,omitempty"`
	LastGC       string `json:"lastGC,omitempty"`
}

type oomReporter struct{}

var _ oomkiller.OOMReporter = (*oomReporter)(nil)

func (r *oomReporter) WriteReport(memoryUsage uint64) error {
	draftPath := filepath.Join(sWorkingPath, "oom_draft")
	draftInfo, err := os.Stat(draftPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		draftInfo = nil
	}
	reportsDir := filepath.Join(sWorkingPath, "oom_reports")
	err = os.MkdirAll(reportsDir, 0o777)
	if err != nil {
		return err
	}
	chownReport(reportsDir)

	destPath, err := nextAvailableReportPath(reportsDir, time.Now().UTC())
	if err != nil {
		return err
	}
	err = r.writeSnapshot(destPath, memoryUsage)
	if err != nil {
		return err
	}
	return discardDraftIfCurrent(draftPath, draftInfo)
}

func (r *oomReporter) WriteDraft(memoryUsage uint64) error {
	draftPath := filepath.Join(sWorkingPath, "oom_draft")
	os.RemoveAll(draftPath)
	return r.writeSnapshot(draftPath, memoryUsage)
}

func (r *oomReporter) DiscardDraft() error {
	draftPath := filepath.Join(sWorkingPath, "oom_draft")
	return os.RemoveAll(draftPath)
}

func discardDraftIfCurrent(draftPath string, draftInfo os.FileInfo) error {
	if draftInfo == nil {
		return nil
	}
	currentInfo, err := os.Stat(draftPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !os.SameFile(draftInfo, currentInfo) {
		return nil
	}
	return os.RemoveAll(draftPath)
}

func (r *oomReporter) writeSnapshot(destPath string, memoryUsage uint64) error {
	now := time.Now().UTC()
	err := os.MkdirAll(destPath, 0o777)
	if err != nil {
		return err
	}
	chownReport(destPath)

	for _, name := range oomReportProfiles {
		writeOOMProfile(destPath, name)
	}

	writeReportFile(destPath, "cmdline", []byte(strings.Join(os.Args, "\000")))

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metadata := oomReportMetadata{
		reportMetadata: baseReportMetadata(),
		RecordedAt:     now.Format(time.RFC3339),
		MemoryUsage:    byteformats.FormatMemoryBytes(memoryUsage),
		// Heap
		HeapAlloc:    byteformats.FormatMemoryBytes(memStats.HeapAlloc),
		HeapObjects:  memStats.HeapObjects,
		HeapInuse:    byteformats.FormatMemoryBytes(memStats.HeapInuse),
		HeapIdle:     byteformats.FormatMemoryBytes(memStats.HeapIdle),
		HeapReleased: byteformats.FormatMemoryBytes(memStats.HeapReleased),
		HeapSys:      byteformats.FormatMemoryBytes(memStats.HeapSys),
		// Stack
		StackInuse: byteformats.FormatMemoryBytes(memStats.StackInuse),
		StackSys:   byteformats.FormatMemoryBytes(memStats.StackSys),
		// Runtime metadata
		MSpanInuse:  byteformats.FormatMemoryBytes(memStats.MSpanInuse),
		MSpanSys:    byteformats.FormatMemoryBytes(memStats.MSpanSys),
		MCacheSys:   byteformats.FormatMemoryBytes(memStats.MCacheSys),
		BuckHashSys: byteformats.FormatMemoryBytes(memStats.BuckHashSys),
		GCSys:       byteformats.FormatMemoryBytes(memStats.GCSys),
		OtherSys:    byteformats.FormatMemoryBytes(memStats.OtherSys),
		Sys:         byteformats.FormatMemoryBytes(memStats.Sys),
		// GC & runtime
		TotalAlloc:   byteformats.FormatMemoryBytes(memStats.TotalAlloc),
		NumGC:        memStats.NumGC,
		NumGoroutine: runtime.NumGoroutine(),
		NextGC:       byteformats.FormatMemoryBytes(memStats.NextGC),
	}
	if memStats.LastGC > 0 {
		metadata.LastGC = time.Unix(0, int64(memStats.LastGC)).UTC().Format(time.RFC3339)
	}
	availableMemory := memory.Available()
	if availableMemory > 0 {
		metadata.AvailableMemory = byteformats.FormatMemoryBytes(availableMemory)
	}
	writeReportMetadata(destPath, metadata)
	copyConfigSnapshot(destPath)

	return nil
}

func writeOOMProfile(destPath string, name string) {
	filePath, err := oomprofile.WriteFile(destPath, name)
	if err != nil {
		return
	}
	chownReport(filePath)
}

func promoteOOMDraftAt(workingPath string) {
	draftPath := filepath.Join(workingPath, "oom_draft")
	info, err := os.Stat(draftPath)
	if err != nil || !info.IsDir() {
		return
	}
	reportsDir := filepath.Join(workingPath, "oom_reports")
	initReportDir(reportsDir)
	destPath, err := nextAvailableReportPath(reportsDir, info.ModTime().UTC())
	if err != nil {
		os.RemoveAll(draftPath)
		return
	}
	err = os.Rename(draftPath, destPath)
	if err != nil {
		os.RemoveAll(draftPath)
		return
	}
	chownReport(destPath)
}

func promoteOOMDraft() {
	promoteOOMDraftAt(sWorkingPath)
}

func PromoteOOMDraft() {
	promoteOOMDraft()
}

func PromoteOOMDraftAt(workingPath string) {
	promoteOOMDraftAt(workingPath)
}
