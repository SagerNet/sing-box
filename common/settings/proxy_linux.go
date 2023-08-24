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
	"github.com/sagernet/sing/common/shell"
)

var (
	hasGSettings bool
	isKDE5       bool
	sudoUser     string
)

func init() {
	isKDE5 = common.Error(exec.LookPath("kwriteconfig5")) == nil
	hasGSettings = common.Error(exec.LookPath("gsettings")) == nil
	if os.Getuid() == 0 {
		sudoUser = os.Getenv("SUDO_USER")
	}
}

func runAsUser(name string, args ...string) error {
	if os.Getuid() != 0 {
		return shell.Exec(name, args...).Attach().Run()
	} else if sudoUser != "" {
		return shell.Exec("su", "-", sudoUser, "-c", F.ToString(name, " ", strings.Join(args, " "))).Attach().Run()
	} else {
		return E.New("set system proxy: unable to set as root")
	}
}

func SetSystemProxy(router adapter.Router, port uint16, isMixed bool) (func() error, error) {
	if hasGSettings {
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
	if isKDE5 {
		err := runAsUser("kwriteconfig5", "--file", "kioslaverc", "--group", "'Proxy Settings'", "--key", "ProxyType", "1")
		if err != nil {
			return nil, err
		}
		if isMixed {
			err = setKDEProxy(port, "ftp", "http", "https", "socks")
		} else {
			err = setKDEProxy(port, "http", "https")
		}
		if err != nil {
			return nil, err
		}
		err = runAsUser("kwriteconfig5", "--file", "kioslaverc", "--group", "'Proxy Settings'", "--key", "Authmode", "0")
		if err != nil {
			return nil, err
		}
		err = runAsUser("dbus-send", "--type=signal", "/KIO/Scheduler", "org.kde.KIO.Scheduler.reparseSlaveConfiguration", "string:''")
		if err != nil {
			return nil, err
		}
		return func() error {
			err = runAsUser("kwriteconfig5", "--file", "kioslaverc", "--group", "'Proxy Settings'", "--key", "ProxyType", "0")
			if err != nil {
				return err
			}
			return runAsUser("dbus-send", "--type=signal", "/KIO/Scheduler", "org.kde.KIO.Scheduler.reparseSlaveConfiguration", "string:''")
		}, nil
	}
	return nil, E.New("unsupported desktop environment")
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

func setKDEProxy(port uint16, proxyTypes ...string) error {
	for _, proxyType := range proxyTypes {
		var proxyUrl string
		if proxyType == "socks" {
			proxyUrl = "socks://127.0.0.1:" + F.ToString(port)
		} else {
			proxyUrl = "http://127.0.0.1:" + F.ToString(port)
		}
		err := runAsUser(
			"kwriteconfig5",
			"--file",
			"kioslaverc",
			"--group",
			"'Proxy Settings'",
			"--key", proxyType+"Proxy",
			proxyUrl,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
