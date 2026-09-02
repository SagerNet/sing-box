package oomkiller

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

func PromoteDraft(basePath string, accept func(metadataContent []byte) bool) {
	draftPath := filepath.Join(basePath, DraftDirectoryName)
	info, err := os.Stat(draftPath)
	if err != nil || !info.IsDir() {
		return
	}
	lockPath := filepath.Join(draftPath, lockFileName)
	lock, err := lockDraft(lockPath)
	if err != nil {
		return
	}
	os.Remove(lockPath)
	unlockDraft(lock)
	if !draftNotable(draftPath) {
		os.RemoveAll(draftPath)
		return
	}
	if accept != nil {
		metadataContent, readErr := os.ReadFile(filepath.Join(draftPath, metadataFileName))
		if readErr != nil || !accept(metadataContent) {
			os.RemoveAll(draftPath)
			return
		}
	}
	promoteDirectory(draftPath, filepath.Join(basePath, ReportsDirectoryName))
}

func draftNotable(draftPath string) bool {
	file, err := os.Open(filepath.Join(draftPath, eventsFileName))
	if err != nil {
		return false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		var event struct {
			Type string `json:"t"`
		}
		err = json.Unmarshal(scanner.Bytes(), &event)
		if err != nil {
			continue
		}
		switch event.Type {
		case eventTypePressure, eventTypeReset, eventTypeSnapshot:
			return true
		}
	}
	return false
}

func promoteDirectory(draftPath string, reportsPath string) {
	info, err := os.Stat(draftPath)
	if err != nil || !info.IsDir() {
		return
	}
	err = os.MkdirAll(reportsPath, 0o777)
	if err != nil {
		return
	}
	destPath, err := nextAvailableReportPath(reportsPath, info.ModTime().UTC())
	if err != nil {
		os.RemoveAll(draftPath)
		return
	}
	err = os.Rename(draftPath, destPath)
	if err != nil {
		os.RemoveAll(draftPath)
	}
}

func nextAvailableReportPath(reportsDir string, timestamp time.Time) (string, error) {
	destName := timestamp.Format("2006-01-02T15-04-05")
	destPath := filepath.Join(reportsDir, destName)
	_, err := os.Stat(destPath)
	if os.IsNotExist(err) {
		return destPath, nil
	}
	for i := 1; i <= 1000; i++ {
		suffixedPath := filepath.Join(reportsDir, destName+"-"+strconv.Itoa(i))
		_, err = os.Stat(suffixedPath)
		if os.IsNotExist(err) {
			return suffixedPath, nil
		}
	}
	return "", E.New("no available report path for ", destName)
}

func copyDirectory(sourcePath string, destPath string) error {
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return err
	}
	err = os.MkdirAll(destPath, 0o777)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		err = copyFile(filepath.Join(sourcePath, entry.Name()), filepath.Join(destPath, entry.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

func copyFile(sourcePath string, destPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	dest, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
	if err != nil {
		return err
	}
	_, err = io.Copy(dest, source)
	if err != nil {
		dest.Close()
		return err
	}
	return dest.Close()
}
