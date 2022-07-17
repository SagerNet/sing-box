package settings

import (
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/wininet"
)

func ClearSystemProxy() error {
	return wininet.ClearSystemProxy()
}

func SetSystemProxy(port uint16, mixed bool) error {
	return wininet.SetSystemProxy(F.ToString("http://127.0.0.1:", port), "local")
}
