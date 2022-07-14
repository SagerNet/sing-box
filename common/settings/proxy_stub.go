//go:build !windows && !linux

package settings

import "github.com/sagernet/sing-box/log"

func ClearSystemProxy() error {
	return nil
}

func SetSystemProxy(port uint16, mixed bool) error {
	log.Warn("set system proxy: unsupported operating system")
	return nil
}
