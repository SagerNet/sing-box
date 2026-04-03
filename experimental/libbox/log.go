//go:build darwin || linux

package libbox

import (
	"archive/zip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"
)

var crashOutputFile *os.File

type crashReportMetadata struct {
	reportMetadata
	CrashedAt       string `json:"crashedAt,omitempty"`
	SignalName      string `json:"signalName,omitempty"`
	SignalCode      string `json:"signalCode,omitempty"`
	ExceptionName   string `json:"exceptionName,omitempty"`
	ExceptionReason string `json:"exceptionReason,omitempty"`
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

	initReportDir(crashReportsDir)
	destPath := nextAvailableReportPath(crashReportsDir, crashTime)
	initReportDir(destPath)

	writeReportFile(destPath, "go.log", content)
	metadata := crashReportMetadata{
		reportMetadata: baseReportMetadata(),
		CrashedAt:      crashTime.Format(time.RFC3339),
	}
	writeReportMetadata(destPath, metadata)
	os.Remove(path)
	copyConfigSnapshot(destPath)
	os.Remove(configSnapshotPath())
}

func configSnapshotPath() string {
	return filepath.Join(sTempPath, "configuration.json")
}

func saveConfigSnapshot(configContent string) {
	snapshotPath := configSnapshotPath()
	os.WriteFile(snapshotPath, []byte(configContent), 0o666)
	chownReport(snapshotPath)
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
