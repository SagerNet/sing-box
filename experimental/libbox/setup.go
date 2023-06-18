package libbox

import (
	"os"

	C "github.com/sagernet/sing-box/constant"

	"github.com/dustin/go-humanize"
)

var (
	sBasePath string
	sTempPath string
	sUserID   int
	sGroupID  int
)

func Setup(basePath string, tempPath string, userID int, groupID int) {
	sBasePath = basePath
	sTempPath = tempPath
	sUserID = userID
	sGroupID = groupID
	if sUserID == -1 {
		sUserID = os.Getuid()
	}
	if sGroupID == -1 {
		sGroupID = os.Getgid()
	}
}

func Version() string {
	return C.Version
}

func FormatBytes(length int64) string {
	return humanize.IBytes(uint64(length))
}

type Func interface {
	Invoke() error
}

type BoolFunc interface {
	Invoke() bool
}
