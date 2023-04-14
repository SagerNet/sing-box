package constant

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sagernet/sing/common/rw"
)

const dirName = "sing-box"

var (
	basePath      string
	tempPath      string
	resourcePaths []string
)

func BasePath(name string) string {
	if basePath == "" || strings.HasPrefix(name, "/") {
		return name
	}
	return filepath.Join(basePath, name)
}

func CreateTemp(pattern string) (*os.File, error) {
	if tempPath == "" {
		tempPath = os.TempDir()
	}
	return os.CreateTemp(tempPath, pattern)
}

func SetBasePath(path string) {
	basePath = path
}

func SetTempPath(path string) {
	tempPath = path
}

func FindPath(name string) (string, bool) {
	name = os.ExpandEnv(name)
	if rw.FileExists(name) {
		return name, true
	}
	for _, dir := range resourcePaths {
		if path := filepath.Join(dir, dirName, name); rw.FileExists(path) {
			return path, true
		}
		if path := filepath.Join(dir, name); rw.FileExists(path) {
			return path, true
		}
	}
	return name, false
}

func init() {
	resourcePaths = append(resourcePaths, ".")
	if home := os.Getenv("HOME"); home != "" {
		resourcePaths = append(resourcePaths, home)
	}
	if userConfigDir, err := os.UserConfigDir(); err == nil {
		resourcePaths = append(resourcePaths, userConfigDir)
	}
	if userCacheDir, err := os.UserCacheDir(); err == nil {
		resourcePaths = append(resourcePaths, userCacheDir)
	}
}
