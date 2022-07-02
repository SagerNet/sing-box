//go:build unix

package constant

import (
	"os"
)

func init() {
	resourcePaths = append(resourcePaths, "/etc/config")
	resourcePaths = append(resourcePaths, "/usr/share")
	resourcePaths = append(resourcePaths, "/usr/local/etc/config")
	resourcePaths = append(resourcePaths, "/usr/local/share")
	if homeDir := os.Getenv("HOME"); homeDir != "" {
		resourcePaths = append(resourcePaths, homeDir+"/.local/share")
	}
}
