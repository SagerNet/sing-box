//go:build darwin || linux || windows

package libbox

import (
	"time"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/service/powerreport"
)

type powerReportMetadata struct {
	reportMetadata
	StartedAt string `json:"startedAt"`
}

func PowerReportOptions(startedService *daemon.StartedService) powerreport.Options {
	return powerreport.Options{
		BasePath: sWorkingPath,
		Logger:   log.StdLogger(),
		Metadata: powerReportMetadata{
			reportMetadata: baseReportMetadata(),
			StartedAt:      time.Now().UTC().Format(time.RFC3339),
		},
		OwnerCallback: chownReport,
		LogCallback: func() []byte {
			return formatLogEntries(startedService.SavedLog())
		},
		ProfileCallback: func(path string) {
			for _, name := range oomReportProfiles {
				writeOOMProfile(path, name)
			}
		},
	}
}

func PromotePowerReportDraft() {
	powerreport.PromoteDraft(sWorkingPath)
}
