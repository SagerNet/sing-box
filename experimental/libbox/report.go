//go:build darwin || linux

package libbox

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	C "github.com/sagernet/sing-box/constant"
)

type reportMetadata struct {
	Source              string `json:"source,omitempty"`
	BundleIdentifier    string `json:"bundleIdentifier,omitempty"`
	ProcessName         string `json:"processName,omitempty"`
	ProcessPath         string `json:"processPath,omitempty"`
	StartedAt           string `json:"startedAt,omitempty"`
	AppVersion          string `json:"appVersion,omitempty"`
	AppMarketingVersion string `json:"appMarketingVersion,omitempty"`
	CoreVersion         string `json:"coreVersion,omitempty"`
	GoVersion           string `json:"goVersion,omitempty"`
}

func baseReportMetadata() reportMetadata {
	processPath, _ := os.Executable()
	processName := filepath.Base(processPath)
	if processName == "." {
		processName = ""
	}
	return reportMetadata{
		Source:      sCrashReportSource,
		ProcessName: processName,
		ProcessPath: processPath,
		CoreVersion: C.Version,
		GoVersion:   GoVersion(),
	}
}

func writeReportFile(destPath string, name string, content []byte) {
	filePath := filepath.Join(destPath, name)
	os.WriteFile(filePath, content, 0o666)
	chownReport(filePath)
}

func writeReportMetadata(destPath string, metadata any) {
	data, err := json.Marshal(metadata)
	if err != nil {
		return
	}
	writeReportFile(destPath, "metadata.json", data)
}

func copyConfigSnapshot(destPath string) {
	snapshotPath := configSnapshotPath()
	content, err := os.ReadFile(snapshotPath)
	if err != nil || len(bytes.TrimSpace(content)) == 0 {
		return
	}
	writeReportFile(destPath, "configuration.json", content)
}

func initReportDir(path string) {
	os.MkdirAll(path, 0o777)
	chownReport(path)
}

func chownReport(path string) {
	if runtime.GOOS != "android" {
		os.Chown(path, sUserID, sGroupID)
	}
}

func nextAvailableReportPath(reportsDir string, timestamp time.Time) string {
	destName := timestamp.Format("2006-01-02T15-04-05")
	destPath := filepath.Join(reportsDir, destName)
	for i := 1; i <= 1000; i++ {
		_, err := os.Stat(destPath)
		if os.IsNotExist(err) {
			break
		}
		destPath = filepath.Join(reportsDir, destName+"-"+strconv.Itoa(i))
	}
	return destPath
}
