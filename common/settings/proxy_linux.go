//go:build linux && !android

package settings

import (
	"os"
	"os/exec"
	"strings"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

var (
	hasGSettings bool
	sudoUser     string
)

func init() {
	hasGSettings = common.Error(exec.LookPath("gsettings")) == nil
	if os.Getuid() == 0 {
		sudoUser = os.Getenv("SUDO_USER")
	}
}

func runAsUser(name string, args ...string) error {
	if os.Getuid() != 0 {
		return runCommand(name, args...)
	} else if sudoUser != "" {
		return runCommand("su", "-", sudoUser, "-c", F.ToString(name, " ", strings.Join(args, " ")))
	} else {
		return E.New("set system proxy: unable to set as root")
	}
}

func ClearSystemProxy() error {
	if hasGSettings {
		return runAsUser("gsettings", "set", "org.gnome.system.proxy", "mode", "none")
	}
	return nil
}

func SetSystemProxy(port uint16, mixed bool) error {
	if hasGSettings {
		err := runAsUser("gsettings", "set", "org.gnome.system.proxy.http", "enabled", "true")
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
		err = runAsUser("gsettings", "set", "org.gnome.system.proxy", "use-same-proxy", F.ToString(mixed))
		if err != nil {
			return err
		}
		err = runAsUser("gsettings", "set", "org.gnome.system.proxy", "mode", "manual")
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
		err := runAsUser("gsettings", "set", "org.gnome.system.proxy."+proxyType, "host", "127.0.0.1")
		if err != nil {
			return err
		}
		err = runAsUser("gsettings", "set", "org.gnome.system.proxy."+proxyType, "port", F.ToString(port))
		if err != nil {
			return err
		}
	}
	return nil
}
