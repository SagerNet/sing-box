//go:build darwin || linux || windows

package libbox

import (
	"path/filepath"
	"time"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/service/powerreport"
)

type powerReportMetadata struct {
	reportMetadata
	StartedAt          string `json:"startedAt"`
	IncludeAllNetworks *bool  `json:"includeAllNetworks,omitempty"`
}

func PowerReportOptions(startedService *daemon.StartedService, platformInterface PlatformInterface) powerreport.Options {
	metadata := powerReportMetadata{
		reportMetadata: baseReportMetadata(),
		StartedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	if platformInterface != nil && platformInterface.UnderNetworkExtension() {
		includeAllNetworks := platformInterface.IncludeAllNetworks()
		metadata.IncludeAllNetworks = &includeAllNetworks
	}
	return powerreport.Options{
		BasePath:      sWorkingPath,
		Logger:        log.StdLogger(),
		Metadata:      metadata,
		OwnerCallback: chownReport,
		LogCallback: func() []byte {
			return formatLogEntries(startedService.SavedLog())
		},
		ProfileCallback: func(path string) {
			for _, name := range oomReportProfiles {
				writeOOMProfile(filepath.Join(path, name+".pb"), name)
			}
		},
	}
}

func DiscardPowerReportDraft() {
	powerreport.DiscardDraft(sWorkingPath)
}
