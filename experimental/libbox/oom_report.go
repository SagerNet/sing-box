//go:build darwin || linux || windows

package libbox

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/sagernet/sing-box/common/trafficcontrol"
	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/experimental/libbox/internal/oomprofile"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/service/oomkiller"
	"github.com/sagernet/sing/common/byteformats"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/memory"
)

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

type oomReporter struct {
	startedService *daemon.StartedService
}

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
	writeOOMLog(destPath, r.startedService.SavedLog())
	r.writeOOMConnections(destPath)

	return nil
}

type oomConnectionsInfo struct {
	UploadTotal       string              `json:"uploadTotal,omitempty"`
	DownloadTotal     string              `json:"downloadTotal,omitempty"`
	Connections       []oomConnectionInfo `json:"connections"`
	ClosedConnections []oomConnectionInfo `json:"closedConnections,omitempty"`
}

type oomConnectionInfo struct {
	ID           string   `json:"id"`
	CreatedAt    string   `json:"createdAt"`
	ClosedAt     string   `json:"closedAt,omitempty"`
	Inbound      string   `json:"inbound,omitempty"`
	Network      string   `json:"network,omitempty"`
	Source       string   `json:"source,omitempty"`
	Destination  string   `json:"destination,omitempty"`
	Host         string   `json:"host,omitempty"`
	User         string   `json:"user,omitempty"`
	Process      string   `json:"process,omitempty"`
	Rule         string   `json:"rule,omitempty"`
	Chain        []string `json:"chain,omitempty"`
	Outbound     string   `json:"outbound,omitempty"`
	OutboundType string   `json:"outboundType,omitempty"`
	Upload       string   `json:"upload,omitempty"`
	Download     string   `json:"download,omitempty"`
}

func (r *oomReporter) writeOOMConnections(destPath string) {
	instance := r.startedService.Instance()
	if instance == nil {
		return
	}
	trafficManager := instance.TrafficManager()
	if trafficManager == nil {
		return
	}
	connections := trafficManager.Connections()
	sort.Slice(connections, func(i, j int) bool {
		return connections[i].CreatedAt.Before(connections[j].CreatedAt)
	})
	uploadTotal, downloadTotal := trafficManager.Total()
	info := oomConnectionsInfo{
		UploadTotal:       byteformats.FormatBytes(uint64(uploadTotal)),
		DownloadTotal:     byteformats.FormatBytes(uint64(downloadTotal)),
		Connections:       buildOOMConnections(connections),
		ClosedConnections: buildOOMConnections(trafficManager.ClosedConnections()),
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return
	}
	writeReportFile(destPath, "connections.json", data)
}

func buildOOMConnections(connections []*trafficcontrol.TrackerMetadata) []oomConnectionInfo {
	result := make([]oomConnectionInfo, 0, len(connections))
	for _, connection := range connections {
		result = append(result, buildOOMConnection(connection))
	}
	return result
}

func buildOOMConnection(connection *trafficcontrol.TrackerMetadata) oomConnectionInfo {
	metadata := connection.Metadata
	var inbound string
	if metadata.Inbound != "" {
		inbound = metadata.InboundType + "/" + metadata.Inbound
	} else {
		inbound = metadata.InboundType
	}
	var process string
	if processInfo := metadata.ProcessInfo; processInfo != nil {
		if processInfo.ProcessPath != "" {
			process = processInfo.ProcessPath
		} else if len(processInfo.AndroidPackageNames) > 0 {
			process = processInfo.AndroidPackageNames[0]
		}
		if process == "" {
			if processInfo.UserId != -1 {
				process = F.ToString(processInfo.UserId)
			}
		} else if processInfo.UserName != "" {
			process = F.ToString(process, " (", processInfo.UserName, ")")
		} else if processInfo.UserId != -1 {
			process = F.ToString(process, " (", processInfo.UserId, ")")
		}
	}
	var rule string
	if connection.Rule != nil {
		rule = F.ToString(connection.Rule, " => ", connection.Rule.Action())
	} else {
		rule = "final"
	}
	info := oomConnectionInfo{
		ID:           connection.ID.String(),
		CreatedAt:    connection.CreatedAt.UTC().Format(time.RFC3339),
		Inbound:      inbound,
		Network:      metadata.Network,
		Source:       metadata.Source.String(),
		Destination:  metadata.Destination.String(),
		Host:         metadata.Domain,
		User:         metadata.User,
		Process:      process,
		Rule:         rule,
		Chain:        connection.Chain,
		Outbound:     connection.Outbound,
		OutboundType: connection.OutboundType,
		Upload:       byteformats.FormatBytes(uint64(connection.Upload.Load())),
		Download:     byteformats.FormatBytes(uint64(connection.Download.Load())),
	}
	if !connection.ClosedAt.IsZero() {
		info.ClosedAt = connection.ClosedAt.UTC().Format(time.RFC3339)
	}
	return info
}

func writeOOMLog(destPath string, entries []*log.Entry) {
	if len(entries) == 0 {
		return
	}
	var buffer bytes.Buffer
	for _, entry := range entries {
		writeWithoutColors(&buffer, entry.Message)
		buffer.WriteByte('\n')
	}
	writeReportFile(destPath, "go.log", buffer.Bytes())
}

func writeWithoutColors(buffer *bytes.Buffer, message string) {
	start := 0
	for index := 0; index < len(message); {
		if message[index] != '\x1b' || index+1 >= len(message) || message[index+1] != '[' {
			index++
			continue
		}
		end := index + 2
		for end < len(message) && message[end] != 'm' {
			end++
		}
		if end >= len(message) {
			break
		}
		buffer.WriteString(message[start:index])
		index = end + 1
		start = index
	}
	buffer.WriteString(message[start:])
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
