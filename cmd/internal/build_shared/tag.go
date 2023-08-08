package build_shared

import (
	"github.com/sagernet/sing-box/common/badversion"
	"github.com/sagernet/sing/common/shell"
)

func ReadTag() (string, error) {
	currentTag, err := shell.Exec("git", "describe", "--tags").ReadOutput()
	if err != nil {
		return currentTag, err
	}
	currentTagRev, _ := shell.Exec("git", "describe", "--tags", "--abbrev=0").ReadOutput()
	if currentTagRev == currentTag {
		return currentTag[1:], nil
	}
	shortCommit, _ := shell.Exec("git", "rev-parse", "--short", "HEAD").ReadOutput()
	version := badversion.Parse(currentTagRev[1:])
	if version.PreReleaseIdentifier == "" {
		version.Patch++
	}
	return version.String() + "-" + shortCommit, nil
}
