package libbox

import (
	"os"
	"os/user"
	"strconv"

	C "github.com/sagernet/sing-box/constant"

	"github.com/dustin/go-humanize"
)

var (
	sBasePath    string
	sWorkingPath string
	sTempPath    string
	sUserID      int
	sGroupID     int
	sTVOS        bool
)

func Setup(basePath string, workingPath string, tempPath string, isTVOS bool) {
	sBasePath = basePath
	sWorkingPath = workingPath
	sTempPath = tempPath
	sUserID = os.Getuid()
	sGroupID = os.Getgid()
	sTVOS = isTVOS
}

func SetupWithUsername(basePath string, workingPath string, tempPath string, username string) error {
	sBasePath = basePath
	sWorkingPath = workingPath
	sTempPath = tempPath
	sUser, err := user.Lookup(username)
	if err != nil {
		return err
	}
	sUserID, _ = strconv.Atoi(sUser.Uid)
	sGroupID, _ = strconv.Atoi(sUser.Gid)
	return nil
}

func Version() string {
	return C.Version
}

func FormatBytes(length int64) string {
	return humanize.IBytes(uint64(length))
}

func ProxyDisplayType(proxyType string) string {
	return C.ProxyDisplayName(proxyType)
}
