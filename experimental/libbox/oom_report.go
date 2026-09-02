//go:build darwin || linux || windows

package libbox

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"sort"
	"time"

	"github.com/sagernet/sing-box/common/trafficcontrol"
	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/experimental/libbox/internal/oomprofile"
	"github.com/sagernet/sing-box/experimental/libbox/internal/runtimeinfo"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/service/oomkiller"
	"github.com/sagernet/sing/common/byteformats"
	F "github.com/sagernet/sing/common/format"
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
	EndedAt         string `json:"endedAt,omitempty"`
	MemoryLimit     string `json:"memoryLimit,omitempty"`
	MemoryUsage     string `json:"memoryUsage"`
	AvailableMemory string `json:"availableMemory,omitempty"`
	Snapshots       int    `json:"snapshots,omitempty,string"`
}

func OOMRecorderOptions(startedService *daemon.StartedService) oomkiller.RecorderOptions {
	return oomkiller.RecorderOptions{
		BasePath:    sWorkingPath,
		Logger:      log.StdLogger(),
		AcceptDraft: acceptOOMDraft,
		MetadataCallback: func(status oomkiller.ReportStatus) any {
			metadata := oomReportMetadata{
				reportMetadata: baseReportMetadata(),
				RecordedAt:     status.RecordedAt.UTC().Format(time.RFC3339),
				MemoryUsage:    byteformats.FormatMemoryBytes(status.PeakMemory),
				Snapshots:      status.Snapshots,
			}
			metadata.StartedAt = status.StartedAt.UTC().Format(time.RFC3339)
			if !status.EndedAt.IsZero() {
				metadata.EndedAt = status.EndedAt.UTC().Format(time.RFC3339)
			}
			if status.MemoryLimit > 0 {
				metadata.MemoryLimit = byteformats.FormatMemoryBytes(status.MemoryLimit)
			}
			if status.AvailableKnown {
				metadata.AvailableMemory = byteformats.FormatMemoryBytes(status.MinAvailable)
			}
			return metadata
		},
		OwnerCallback: chownReport,
		LogCallback: func() []byte {
			return formatLogEntries(startedService.SavedLog())
		},
		SnapshotCallback: func(directory string, prefix string) {
			for _, name := range oomReportProfiles {
				writeOOMProfile(filepath.Join(directory, prefix+"."+name+".pb"), name)
			}
			runtimeInfoPath := filepath.Join(directory, prefix+".runtime.json")
			err := runtimeinfo.WriteFile(runtimeInfoPath)
			if err == nil {
				chownReport(runtimeInfoPath)
			}
			copyConfigSnapshot(directory)
			content := oomConnectionsContent(startedService)
			if content != nil {
				writeReportFile(directory, prefix+".connections.json", content)
			}
		},
	}
}

func acceptOOMDraft(metadataContent []byte) bool {
	var draftMetadata reportMetadata
	err := json.Unmarshal(metadataContent, &draftMetadata)
	if err != nil {
		return false
	}
	return draftMetadata.AppVersion == sAppVersion && draftMetadata.AppMarketingVersion == sAppMarketingVersion
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

func oomConnectionsContent(startedService *daemon.StartedService) []byte {
	instance := startedService.Instance()
	if instance == nil {
		return nil
	}
	trafficManager := instance.TrafficManager()
	if trafficManager == nil {
		return nil
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
		return nil
	}
	return data
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

func formatLogEntries(entries []*log.Entry) []byte {
	if len(entries) == 0 {
		return nil
	}
	var buffer bytes.Buffer
	for _, entry := range entries {
		writeWithoutColors(&buffer, entry.Message)
		buffer.WriteByte('\n')
	}
	return buffer.Bytes()
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

func writeOOMProfile(filePath string, name string) {
	err := oomprofile.WriteFile(filePath, name)
	if err != nil {
		return
	}
	chownReport(filePath)
}

func PromoteOOMDraft() {
	oomkiller.PromoteDraft(sWorkingPath, acceptOOMDraft)
}

func PromoteOOMDraftAt(workingPath string) {
	oomkiller.PromoteDraft(workingPath, acceptOOMDraft)
}
