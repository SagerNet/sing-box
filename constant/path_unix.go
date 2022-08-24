//go:build unix || linux

package constant

import (
	"os"
)

func init() {
	resourcePaths = append(resourcePaths, "/etc")
	resourcePaths = append(resourcePaths, "/usr/share")
	resourcePaths = append(resourcePaths, "/usr/local/etc")
	resourcePaths = append(resourcePaths, "/usr/local/share")
	if homeDir := os.Getenv("HOME"); homeDir != "" {
		resourcePaths = append(resourcePaths, homeDir+"/.local/share")
	}
}
