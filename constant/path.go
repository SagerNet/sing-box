package constant

import (
	"os"
	"path/filepath"

	"github.com/sagernet/sing/common/rw"
)

const dirName = "sing-box"

var resourcePaths []string

func FindPath(name string) (string, bool) {
	name = os.ExpandEnv(name)
	if rw.FileExists(name) {
		return name, true
	}
	for _, dir := range resourcePaths {
		if path := filepath.Join(dir, dirName, name); rw.FileExists(path) {
			return path, true
		}
	}
	return name, false
}

func init() {
	resourcePaths = append(resourcePaths, ".")
	if userConfigDir, err := os.UserConfigDir(); err == nil {
		resourcePaths = append(resourcePaths, userConfigDir)
	}
	if userCacheDir, err := os.UserCacheDir(); err == nil {
		resourcePaths = append(resourcePaths, userCacheDir)
	}
}
