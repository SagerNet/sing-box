package settings

import (
	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/wininet"
)

func SetSystemProxy(router adapter.Router, port uint16, isMixed bool) (func() error, error) {
	err := wininet.SetSystemProxy(F.ToString("http://127.0.0.1:", port), "<local>")
	if err != nil {
		return nil, err
	}
	return func() error {
		return wininet.ClearSystemProxy()
	}, nil
}
