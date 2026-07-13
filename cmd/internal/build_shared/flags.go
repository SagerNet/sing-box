package build_shared

import "strings"

func LinkerFlags(version string, debug bool) string {
	flags := []string{
		"-X github.com/sagernet/sing-box/constant.Version=" + version,
		"-X internal/godebug.defaultGODEBUG=multipathtcp=0",
		"-checklinkname=0",
	}
	if !debug {
		flags = append(flags, "-s", "-w", "-buildid=")
	}
	return strings.Join(flags, " ")
}
