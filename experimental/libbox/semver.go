package libbox

import (
	"strings"

	"golang.org/x/mod/semver"
)

func CompareSemver(left string, right string) bool {
	normalizedLeft := normalizeSemver(left)
	if !semver.IsValid(normalizedLeft) {
		return false
	}
	normalizedRight := normalizeSemver(right)
	if !semver.IsValid(normalizedRight) {
		return false
	}
	return semver.Compare(normalizedLeft, normalizedRight) > 0
}

func normalizeSemver(version string) string {
	trimmedVersion := strings.TrimSpace(version)
	if strings.HasPrefix(trimmedVersion, "v") {
		return trimmedVersion
	}
	return "v" + trimmedVersion
}
