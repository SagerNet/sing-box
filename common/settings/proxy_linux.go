//go:build linux && !android

package settings

import (
	"os"
	"os/exec"
	"strings"

	"github.com/sagernet/sing-box/adapter"
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
		return common.Exec(name, args...).Attach().Run()
	} else if sudoUser != "" {
		return common.Exec("su", "-", sudoUser, "-c", F.ToString(name, " ", strings.Join(args, " "))).Attach().Run()
	} else {
		return E.New("set system proxy: unable to set as root")
	}
}

func SetSystemProxy(router adapter.Router, port uint16, isMixed bool) (func() error, error) {
	if !hasGSettings {
		return nil, E.New("unsupported desktop environment")
	}
	err := runAsUser("gsettings", "set", "org.gnome.system.proxy.http", "enabled", "true")
	if err != nil {
		return nil, err
	}
	if isMixed {
		err = setGnomeProxy(port, "ftp", "http", "https", "socks")
	} else {
		err = setGnomeProxy(port, "http", "https")
	}
	if err != nil {
		return nil, err
	}
	err = runAsUser("gsettings", "set", "org.gnome.system.proxy", "use-same-proxy", F.ToString(isMixed))
	if err != nil {
		return nil, err
	}
	err = runAsUser("gsettings", "set", "org.gnome.system.proxy", "mode", "manual")
	if err != nil {
		return nil, err
	}
	return func() error {
		return runAsUser("gsettings", "set", "org.gnome.system.proxy", "mode", "none")
	}, nil
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
