//go:build with_tailscale

package tailssh

import (
	"slices"
	"strings"
)

const maxAcceptEnvNameLen = 256

func acceptClientEnv(pair string, acceptEnv []string) bool {
	name, value, found := strings.Cut(pair, "=")
	if !found || len(name) > maxAcceptEnvNameLen || strings.ContainsRune(pair, 0) || isDangerousEnvVar(name) || forbiddenEnvKey(name) {
		return false
	}
	switch {
	case name == "USER", name == "LOGNAME", name == "HOME", name == "SHELL", name == "PATH":
		return false
	case name == "TERM":
		if !strings.Contains(value, "/") {
			return true
		}
	case name == "LANG", strings.HasPrefix(name, "LC_"):
		if !strings.Contains(value, "/") && !strings.HasPrefix(value, "-") {
			return true
		}
	}
	return slices.ContainsFunc(acceptEnv, func(pattern string) bool {
		return matchAcceptEnvPattern(pattern, name)
	})
}

func isDangerousEnvVar(name string) bool {
	upper := strings.ToUpper(name)
	if strings.HasPrefix(upper, "LD_") || strings.HasPrefix(upper, "DYLD_") {
		return true
	}
	switch upper {
	case "GOTRACEBACK", "GCONV_PATH", "LOCPATH", "NLSPATH", "HOSTALIASES", "RES_OPTIONS", "GLIBC_TUNABLES",
		"IFS", "ENV", "BASH_ENV", "SHELLOPTS", "BASHOPTS", "PS4", "GLOBIGNORE":
		return true
	}
	return false
}

func forbiddenEnvKey(name string) bool {
	if name == "" {
		return true
	}
	for _, r := range name {
		if r != '_' && r != '-' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return true
		}
	}
	return false
}

func matchAcceptEnvPattern(pattern string, target string) bool {
	var (
		patternIndex, targetIndex int
		starPattern               = -1
		starTarget                = -1
	)
	for targetIndex < len(target) {
		switch {
		case patternIndex < len(pattern) && pattern[patternIndex] == '*':
			starPattern = patternIndex
			starTarget = targetIndex
			patternIndex++
		case patternIndex < len(pattern) && (pattern[patternIndex] == '?' || pattern[patternIndex] == target[targetIndex]):
			patternIndex++
			targetIndex++
		case starPattern >= 0:
			patternIndex = starPattern + 1
			starTarget++
			targetIndex = starTarget
		default:
			return false
		}
	}
	for patternIndex < len(pattern) && pattern[patternIndex] == '*' {
		patternIndex++
	}
	return patternIndex == len(pattern)
}
