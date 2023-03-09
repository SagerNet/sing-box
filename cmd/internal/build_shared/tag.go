package build_shared

import (
	"github.com/sagernet/sing/common"
)

func ReadTag() (string, error) {
	currentTag, err := common.Exec("git", "describe", "--tags").ReadOutput()
	if err != nil {
		return currentTag, err
	}
	currentTagRev, _ := common.Exec("git", "describe", "--tags", "--abbrev=0").ReadOutput()
	if currentTagRev == currentTag {
		return currentTag[1:], nil
	}
	shortCommit, _ := common.Exec("git", "rev-parse", "--short", "HEAD").ReadOutput()
	return currentTagRev[1:] + "-" + shortCommit, nil
}
