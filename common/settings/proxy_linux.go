//go:build linux && !android

package settings

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/shell"
)

type LinuxSystemProxy struct {
	hasGSettings    bool
	kWriteConfigCmd string
	sudoUser        string
	serverAddr      M.Socksaddr
	supportSOCKS    bool
	isEnabled       bool
}

func NewSystemProxy(ctx context.Context, serverAddr M.Socksaddr, supportSOCKS bool) (*LinuxSystemProxy, error) {
	hasGSettings := common.Error(exec.LookPath("gsettings")) == nil
	kWriteConfigCmds := []string{
		"kwriteconfig5",
		"kwriteconfig6",
	}
	var kWriteConfigCmd string
	for _, cmd := range kWriteConfigCmds {
		if common.Error(exec.LookPath(cmd)) == nil {
			kWriteConfigCmd = cmd
			break
		}
	}
	var sudoUser string
	if os.Getuid() == 0 {
		sudoUser = os.Getenv("SUDO_USER")
	}
	if !hasGSettings && kWriteConfigCmd == "" {
		return nil, E.New("unsupported desktop environment")
	}
	return &LinuxSystemProxy{
		hasGSettings:    hasGSettings,
		kWriteConfigCmd: kWriteConfigCmd,
		sudoUser:        sudoUser,
		serverAddr:      serverAddr,
		supportSOCKS:    supportSOCKS,
	}, nil
}

func (p *LinuxSystemProxy) IsEnabled() bool {
	return p.isEnabled
}

func (p *LinuxSystemProxy) Enable() error {
	if p.hasGSettings {
		err := p.runAsUser("gsettings", "set", "org.gnome.system.proxy.http", "enabled", "true")
		if err != nil {
			return err
		}
		if p.supportSOCKS {
			err = p.setGnomeProxy("ftp", "http", "https", "socks")
		} else {
			err = p.setGnomeProxy("http", "https")
		}
		if err != nil {
			return err
		}
		err = p.runAsUser("gsettings", "set", "org.gnome.system.proxy", "use-same-proxy", F.ToString(p.supportSOCKS))
		if err != nil {
			return err
		}
		err = p.runAsUser("gsettings", "set", "org.gnome.system.proxy", "mode", "manual")
		if err != nil {
			return err
		}
	}
	if p.kWriteConfigCmd != "" {
		err := p.runAsUser(p.kWriteConfigCmd, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ProxyType", "1")
		if err != nil {
			return err
		}
		if p.supportSOCKS {
			err = p.setKDEProxy("ftp", "http", "https", "socks")
		} else {
			err = p.setKDEProxy("http", "https")
		}
		if err != nil {
			return err
		}
		err = p.runAsUser(p.kWriteConfigCmd, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "Authmode", "0")
		if err != nil {
			return err
		}
		err = p.runAsUser("dbus-send", "--type=signal", "/KIO/Scheduler", "org.kde.KIO.Scheduler.reparseSlaveConfiguration", "string:''")
		if err != nil {
			return err
		}
	}
	p.isEnabled = true
	return nil
}

func (p *LinuxSystemProxy) Disable() error {
	if p.hasGSettings {
		err := p.runAsUser("gsettings", "set", "org.gnome.system.proxy", "mode", "none")
		if err != nil {
			return err
		}
	}
	if p.kWriteConfigCmd != "" {
		err := p.runAsUser(p.kWriteConfigCmd, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ProxyType", "0")
		if err != nil {
			return err
		}
		err = p.runAsUser("dbus-send", "--type=signal", "/KIO/Scheduler", "org.kde.KIO.Scheduler.reparseSlaveConfiguration", "string:''")
		if err != nil {
			return err
		}
	}
	p.isEnabled = false
	return nil
}

func (p *LinuxSystemProxy) runAsUser(name string, args ...string) error {
	if os.Getuid() != 0 {
		return shell.Exec(name, args...).Attach().Run()
	} else if p.sudoUser != "" {
		return shell.Exec("su", "-", p.sudoUser, "-c", F.ToString(name, " ", strings.Join(args, " "))).Attach().Run()
	} else {
		return E.New("set system proxy: unable to set as root")
	}
}

func (p *LinuxSystemProxy) setGnomeProxy(proxyTypes ...string) error {
	for _, proxyType := range proxyTypes {
		err := p.runAsUser("gsettings", "set", "org.gnome.system.proxy."+proxyType, "host", p.serverAddr.AddrString())
		if err != nil {
			return err
		}
		err = p.runAsUser("gsettings", "set", "org.gnome.system.proxy."+proxyType, "port", F.ToString(p.serverAddr.Port))
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *LinuxSystemProxy) setKDEProxy(proxyTypes ...string) error {
	for _, proxyType := range proxyTypes {
		var proxyUrl string
		if proxyType == "socks" {
			proxyUrl = "socks://" + p.serverAddr.String()
		} else {
			proxyUrl = "http://" + p.serverAddr.String()
		}
		err := p.runAsUser(
			p.kWriteConfigCmd,
			"--file",
			"kioslaverc",
			"--group",
			"Proxy Settings",
			"--key", proxyType+"Proxy",
			proxyUrl,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
