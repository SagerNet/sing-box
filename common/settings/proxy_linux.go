//go:build linux && !android

package settings

import (
	"os/exec"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
)

var hasGSettings bool

func init() {
	hasGSettings = common.Error(exec.LookPath("gsettings")) == nil
}

func ClearSystemProxy() error {
	if hasGSettings {
		return runCommand("gsettings", "set", "org.gnome.system.proxy", "mode", "none")
	}
	return nil
}

func SetSystemProxy(port uint16, mixed bool) error {
	if hasGSettings {
		err := runCommand("gsettings", "set", "org.gnome.system.proxy.http", "enabled", "true")
		if err != nil {
			return err
		}
		if mixed {
			err = setGnomeProxy(port, "ftp", "http", "https", "socks")
			if err != nil {
				return err
			}
		} else {
			err = setGnomeProxy(port, "http", "https")
			if err != nil {
				return err
			}
		}
		err = runCommand("gsettings", "set", "org.gnome.system.proxy", "use-same-proxy", F.ToString(mixed))
		if err != nil {
			return err
		}
		err = runCommand("gsettings", "set", "org.gnome.system.proxy", "mode", "manual")
		if err != nil {
			return err
		}
	} else {
		log.Warn("set system proxy: unsupported desktop environment")
	}
	return nil
}

func setGnomeProxy(port uint16, proxyTypes ...string) error {
	for _, proxyType := range proxyTypes {
		err := runCommand("gsettings", "set", "org.gnome.system.proxy."+proxyType, "host", "127.0.0.1")
		if err != nil {
			return err
		}
		err = runCommand("gsettings", "set", "org.gnome.system.proxy."+proxyType, "port", F.ToString(port))
		if err != nil {
			return err
		}
	}
	return nil
}
