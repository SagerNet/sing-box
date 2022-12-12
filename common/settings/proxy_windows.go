package settings

import (
	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/wininet"
)

func SetSystemProxy(router adapter.Router, port uint16, isMixed bool) (func() error, error) {
	err := wininet.SetSystemProxy(F.ToString("127.0.0.1:", port), "localhost;127.*;10.*;172.16.*;172.17.*;172.18.*;172.19.*;172.20.*;172.21.*;172.22.*;172.23.*;172.24.*;172.25.*;172.26.*;172.27.*;172.28.*;172.29.*;172.30.*;172.31.*;192.168.*;<local>")
	if err != nil {
		return nil, err
	}
	return func() error {
		return wininet.ClearSystemProxy()
	}, nil
}
