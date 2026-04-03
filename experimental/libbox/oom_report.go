//go:build darwin || linux

package libbox

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"time"

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
}

type oomReporter struct{}

var _ oomkiller.OOMReporter = (*oomReporter)(nil)

func (r *oomReporter) WriteReport(memoryUsage uint64) error {
	now := time.Now().UTC()
	reportsDir := filepath.Join(sWorkingPath, "oom_reports")
	err := os.MkdirAll(reportsDir, 0o777)
	if err != nil {
		return err
	}
	chownReport(reportsDir)

	destPath := nextAvailableReportPath(reportsDir, now)
	err = os.MkdirAll(destPath, 0o777)
	if err != nil {
		return err
	}
	chownReport(destPath)

	for _, name := range oomReportProfiles {
		writeOOMProfile(destPath, name)
	}

	writeReportFile(destPath, "cmdline", []byte(strings.Join(os.Args, "\000")))
	metadata := oomReportMetadata{
		reportMetadata: baseReportMetadata(),
		RecordedAt:     now.Format(time.RFC3339),
		MemoryUsage:    byteformats.FormatMemoryBytes(memoryUsage),
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
	profile := pprof.Lookup(name)
	if profile == nil {
		return
	}
	filePath := filepath.Join(destPath, name+".pb.gz")
	file, err := os.Create(filePath)
	if err != nil {
		return
	}
	defer file.Close()
	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()
	profile.WriteTo(gzipWriter, 0)
	chownReport(filePath)
}
