package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sagernet/sing-box/experimental/libbox"
	"github.com/sagernet/sing/common/rw"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// File names within a report directory follow the client convention:
	// sing-box-for-apple Library/Shared/CrashReportArchive.swift (ReportArchive)
	// and sing-box experimental/libbox/report.go.
	readMarkerFileName     = ".read"
	ownerMarkerFileName    = ".owner"
	metadataFileName       = "metadata.json"
	configSnapshotFileName = "configuration.json"
	goLogFileName          = "go.log"
	nativeLogFileName      = "native.log"
)

func reportPath(reportsDirectory string, name string) (string, error) {
	if !filepath.IsLocal(name) || name == "." || name != filepath.Base(name) {
		return "", status.Error(codes.InvalidArgument, "invalid report name")
	}
	fullPath := filepath.Join(reportsDirectory, name)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", status.Error(codes.NotFound, "report not found")
		}
		return "", err
	}
	if !info.IsDir() {
		return "", status.Error(codes.NotFound, "report not found")
	}
	return fullPath, nil
}

func reportPathForUser(reportsDirectory string, name string, userID string) (string, error) {
	fullPath, err := reportPath(reportsDirectory, name)
	if err != nil {
		return "", err
	}
	if !reportOwnedBy(fullPath, userID) {
		return "", status.Error(codes.NotFound, "report not found")
	}
	return fullPath, nil
}

func reportOwnedBy(fullPath string, userID string) bool {
	ownerContent, err := os.ReadFile(filepath.Join(fullPath, ownerMarkerFileName))
	return err == nil && strings.TrimSpace(string(ownerContent)) == userID
}

func tagUnownedReports(reportsDirectory string, userID string) error {
	if userID == "" {
		return nil
	}
	entries, err := os.ReadDir(reportsDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ownerPath := filepath.Join(reportsDirectory, entry.Name(), ownerMarkerFileName)
		_, err = os.Stat(ownerPath)
		if err == nil {
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		err = os.WriteFile(ownerPath, []byte(userID+"\n"), 0o600)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Daemon) reportCaller(ctx context.Context, reportsDirectoryName string) (string, string, error) {
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return "", "", err
	}
	d.lifecycleAccess.Lock()
	defer d.lifecycleAccess.Unlock()
	ownerUserID, err := loadOwner()
	if err != nil {
		return "", "", err
	}
	if ownerUserID != identity.UserID {
		return "", "", status.Error(codes.PermissionDenied, "the service is owned by another user")
	}
	reportsDirectory := filepath.Join(userWorkingDirectory(identity.UserID), reportsDirectoryName)
	err = tagUnownedReports(reportsDirectory, identity.UserID)
	if err != nil {
		return "", "", err
	}
	return reportsDirectory, identity.UserID, nil
}

func deleteReportsForUser(reportsDirectory string, userID string) error {
	entries, err := os.ReadDir(reportsDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(reportsDirectory, entry.Name())
		if !reportOwnedBy(fullPath, userID) {
			continue
		}
		err = os.RemoveAll(fullPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func reportTime(fullPath string, timestampKey string) time.Time {
	metadataContent, err := os.ReadFile(filepath.Join(fullPath, metadataFileName))
	if err == nil {
		var metadata map[string]any
		err = json.Unmarshal(metadataContent, &metadata)
		if err == nil {
			if timestamp, isString := metadata[timestampKey].(string); isString && timestamp != "" {
				parsedTime, parseError := time.Parse(time.RFC3339, timestamp)
				if parseError == nil {
					return parsedTime
				}
			}
		}
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func reportIsRead(fullPath string) bool {
	_, err := os.Stat(filepath.Join(fullPath, readMarkerFileName))
	return err == nil
}

func exportReportArchive(reportsDirectory string, name string, userID string, withConfiguration bool, withLog bool, encrypt bool) (*CrashReportArchive, error) {
	fullPath, err := reportPathForUser(reportsDirectory, name, userID)
	if err != nil {
		return nil, err
	}
	tempRoot := filepath.Join(filepath.Dir(reportsDirectory), "temp")
	err = os.MkdirAll(tempRoot, 0o700)
	if err != nil {
		return nil, err
	}
	tempDirectory, err := os.MkdirTemp(tempRoot, "report-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDirectory)
	strippedPath := filepath.Join(tempDirectory, name)
	err = copyDirectory(fullPath, strippedPath)
	if err != nil {
		return nil, err
	}
	os.Remove(filepath.Join(strippedPath, readMarkerFileName))
	os.Remove(filepath.Join(strippedPath, ownerMarkerFileName))
	if !withConfiguration {
		os.Remove(filepath.Join(strippedPath, configSnapshotFileName))
	}
	if !withLog {
		os.Remove(filepath.Join(strippedPath, goLogFileName))
		os.Remove(filepath.Join(strippedPath, nativeLogFileName))
	}
	fileName := name + ".zip"
	if encrypt {
		fileName += ".age"
	}
	archivePath := filepath.Join(tempDirectory, fileName)
	err = libbox.CreateZipArchive(strippedPath, archivePath, encrypt)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(archivePath)
	if err != nil {
		return nil, err
	}
	return &CrashReportArchive{
		FileName: fileName,
		Data:     data,
	}, nil
}

func copyDirectory(sourcePath string, destinationPath string) error {
	err := os.MkdirAll(destinationPath, 0o700)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourceEntryPath := filepath.Join(sourcePath, entry.Name())
		destinationEntryPath := filepath.Join(destinationPath, entry.Name())
		if entry.IsDir() {
			err = copyDirectory(sourceEntryPath, destinationEntryPath)
			if err != nil {
				return err
			}
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return status.Error(codes.FailedPrecondition, "report contains a non-regular file")
		}
		err = rw.CopyFile(sourceEntryPath, destinationEntryPath)
		if err != nil {
			return err
		}
	}
	return nil
}
