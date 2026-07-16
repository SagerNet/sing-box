package libbox

import (
	"strings"

	"github.com/sagernet/sing-box/common/badversion"

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
	return badversion.Parse(normalizedLeft).GreaterThan(badversion.Parse(normalizedRight))
}

func normalizeSemver(version string) string {
	trimmedVersion := strings.TrimSpace(version)
	if strings.HasPrefix(trimmedVersion, "v") {
		return trimmedVersion
	}
	return "v" + trimmedVersion
}
