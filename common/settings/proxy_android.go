package settings

import (
	"os"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
)

var (
	useRish  bool
	rishPath string
)

func init() {
	userId := os.Getuid()
	if userId == 0 || userId == 1000 || userId == 2000 {
		useRish = false
	} else {
		rishPath, useRish = C.FindPath("rish")
	}
}

func runAndroidShell(name string, args ...string) error {
	if !useRish {
		return common.Exec(name, args...).Attach().Run()
	} else {
		return common.Exec("sh", rishPath, "-c", F.ToString(name, " ", strings.Join(args, " "))).Attach().Run()
	}
}

func SetSystemProxy(router adapter.Router, port uint16, isMixed bool) (func() error, error) {
	err := runAndroidShell("settings", "put", "global", "http_proxy", F.ToString("127.0.0.1:", port))
	if err != nil {
		return nil, err
	}
	return func() error {
		return runAndroidShell("settings", "put", "global", "http_proxy", ":0")
	}, nil
}
