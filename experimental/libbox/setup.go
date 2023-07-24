package libbox

import (
	"os"
	"os/user"
	"strconv"

	C "github.com/sagernet/sing-box/constant"

	"github.com/dustin/go-humanize"
)

var (
	sBasePath string
	sTempPath string
	sUserID   int
	sGroupID  int
)

func Setup(basePath string, tempPath string) {
	sBasePath = basePath
	sTempPath = tempPath
	sUserID = os.Getuid()
	sGroupID = os.Getgid()
}

func SetupWithUsername(basePath string, tempPath string, username string) error {
	sBasePath = basePath
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
