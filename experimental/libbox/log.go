//go:build darwin || linux

package libbox

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"

	C "github.com/sagernet/sing-box/constant"
)

const (
	crashReportMetadataFileName = "metadata.json"
	crashReportGoLogFileName    = "go.log"
	crashReportConfigFileName   = "configuration.json"
)

var crashOutputFile *os.File

type crashReportMetadata struct {
	Source              string `json:"source"`
	BundleIdentifier    string `json:"bundleIdentifier,omitempty"`
	ProcessName         string `json:"processName,omitempty"`
	ProcessPath         string `json:"processPath,omitempty"`
	StartedAt           string `json:"startedAt,omitempty"`
	CrashedAt           string `json:"crashedAt,omitempty"`
	AppVersion          string `json:"appVersion,omitempty"`
	AppMarketingVersion string `json:"appMarketingVersion,omitempty"`
	CoreVersion         string `json:"coreVersion,omitempty"`
	GoVersion           string `json:"goVersion,omitempty"`
	SignalName          string `json:"signalName,omitempty"`
	SignalCode          string `json:"signalCode,omitempty"`
	ExceptionName       string `json:"exceptionName,omitempty"`
	ExceptionReason     string `json:"exceptionReason,omitempty"`
}

func archiveCrashReport(path string, crashReportsDir string) {
	content, err := os.ReadFile(path)
	if err != nil || len(content) == 0 {
		return
	}

	info, _ := os.Stat(path)
	crashTime := time.Now().UTC()
	if info != nil {
		crashTime = info.ModTime().UTC()
	}

	metadata := currentCrashReportMetadata(crashTime)
	if len(bytes.TrimSpace(content)) == 0 {
		os.Remove(path)
		return
	}

	os.MkdirAll(crashReportsDir, 0o777)
	destName := crashTime.Format("2006-01-02T15-04-05")
	destPath := filepath.Join(crashReportsDir, destName)
	for i := 1; ; i++ {
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			break
		}
		destPath = filepath.Join(crashReportsDir,
			crashTime.Format("2006-01-02T15-04-05")+"-"+strconv.Itoa(i))
	}

	os.MkdirAll(destPath, 0o777)
	logPath := filepath.Join(destPath, crashReportGoLogFileName)
	os.WriteFile(logPath, content, 0o666)
	if runtime.GOOS != "android" {
		os.Chown(destPath, sUserID, sGroupID)
		os.Chown(logPath, sUserID, sGroupID)
	}
	writeCrashReportMetadata(destPath, metadata)
	os.Remove(path)
	archiveConfigSnapshot(destPath)
}

func configSnapshotPath() string {
	return filepath.Join(sTempPath, crashReportConfigFileName)
}

func saveConfigSnapshot(configContent string) {
	snapshotPath := configSnapshotPath()
	os.WriteFile(snapshotPath, []byte(configContent), 0o666)
	if runtime.GOOS != "android" {
		os.Chown(snapshotPath, sUserID, sGroupID)
	}
}

func archiveConfigSnapshot(destPath string) {
	snapshotPath := configSnapshotPath()
	content, err := os.ReadFile(snapshotPath)
	if err != nil || len(bytes.TrimSpace(content)) == 0 {
		return
	}
	configPath := filepath.Join(destPath, crashReportConfigFileName)
	os.WriteFile(configPath, content, 0o666)
	if runtime.GOOS != "android" {
		os.Chown(configPath, sUserID, sGroupID)
	}
	os.Remove(snapshotPath)
}

func redirectStderr(path string) error {
	crashReportsDir := filepath.Join(sWorkingPath, "crash_reports")
	archiveCrashReport(path, crashReportsDir)
	archiveCrashReport(path+".old", crashReportsDir)

	outputFile, err := os.Create(path)
	if err != nil {
		return err
	}
	if runtime.GOOS != "android" {
		err = outputFile.Chown(sUserID, sGroupID)
		if err != nil {
			outputFile.Close()
			os.Remove(outputFile.Name())
			return err
		}
	}

	err = debug.SetCrashOutput(outputFile, debug.CrashOptions{})
	if err != nil {
		outputFile.Close()
		os.Remove(outputFile.Name())
		return err
	}
	crashOutputFile = outputFile
	return nil
}

func currentCrashReportMetadata(crashTime time.Time) crashReportMetadata {
	processPath, _ := os.Executable()
	processName := filepath.Base(processPath)
	if processName == "." {
		processName = ""
	}
	return crashReportMetadata{
		Source:      sCrashReportSource,
		ProcessName: processName,
		ProcessPath: processPath,
		CrashedAt:   crashTime.Format(time.RFC3339),
		CoreVersion: C.Version,
		GoVersion:   GoVersion(),
	}
}

func writeCrashReportMetadata(reportPath string, metadata crashReportMetadata) {
	data, err := json.Marshal(metadata)
	if err != nil {
		return
	}

	metaPath := filepath.Join(reportPath, crashReportMetadataFileName)
	os.WriteFile(metaPath, data, 0o666)
	if runtime.GOOS != "android" {
		os.Chown(metaPath, sUserID, sGroupID)
	}
}

func CreateZipArchive(sourcePath string, destinationPath string) error {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() {
		return os.ErrInvalid
	}

	destinationFile, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = destinationFile.Close()
	}()

	zipWriter := zip.NewWriter(destinationFile)

	rootName := filepath.Base(sourcePath)
	err = filepath.WalkDir(sourcePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return nil
		}

		archivePath := filepath.ToSlash(filepath.Join(rootName, relativePath))
		if d.IsDir() {
			_, err = zipWriter.Create(archivePath + "/")
			return err
		}

		fileInfo, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return err
		}
		header.Name = archivePath
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		sourceFile, err := os.Open(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(writer, sourceFile)
		closeErr := sourceFile.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
	if err != nil {
		_ = zipWriter.Close()
		return err
	}

	return zipWriter.Close()
}
