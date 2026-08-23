package powerreport

import (
	"os"
	"path/filepath"
	"strconv"
)

func PromoteDraft(basePath string) {
	promoteDirectory(filepath.Join(basePath, DraftDirectoryName), filepath.Join(basePath, ReportsDirectoryName))
}

func finalizeDraft(draftPath string) {
	promoteDirectory(draftPath, filepath.Join(filepath.Dir(draftPath), ReportsDirectoryName))
}

func promoteDirectory(draftPath string, reportsPath string) {
	info, err := os.Stat(draftPath)
	if err != nil || !info.IsDir() {
		return
	}
	entries, err := os.ReadDir(draftPath)
	if err != nil || len(entries) == 0 {
		os.RemoveAll(draftPath)
		return
	}
	err = os.MkdirAll(reportsPath, 0o777)
	if err != nil {
		return
	}
	destName := info.ModTime().UTC().Format("2006-01-02T15-04-05")
	destPath := filepath.Join(reportsPath, destName)
	for i := 1; ; i++ {
		_, err = os.Stat(destPath)
		if os.IsNotExist(err) {
			break
		}
		if i > 1000 {
			os.RemoveAll(draftPath)
			return
		}
		destPath = filepath.Join(reportsPath, destName+"-"+strconv.Itoa(i))
	}
	err = os.Rename(draftPath, destPath)
	if err != nil {
		os.RemoveAll(draftPath)
	}
}
