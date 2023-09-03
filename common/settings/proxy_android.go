package settings

import (
	"context"
	"os"
	"strings"

	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/shell"
)

type AndroidSystemProxy struct {
	useRish      bool
	rishPath     string
	serverAddr   M.Socksaddr
	supportSOCKS bool
	isEnabled    bool
}

func NewSystemProxy(ctx context.Context, serverAddr M.Socksaddr, supportSOCKS bool) (*AndroidSystemProxy, error) {
	userId := os.Getuid()
	var (
		useRish  bool
		rishPath string
	)
	if userId == 0 || userId == 1000 || userId == 2000 {
		useRish = false
	} else {
		rishPath, useRish = C.FindPath("rish")
		if !useRish {
			return nil, E.Cause(os.ErrPermission, "root or system (adb) permission is required for set system proxy")
		}
	}
	return &AndroidSystemProxy{
		useRish:      useRish,
		rishPath:     rishPath,
		serverAddr:   serverAddr,
		supportSOCKS: supportSOCKS,
	}, nil
}

func (p *AndroidSystemProxy) IsEnabled() bool {
	return p.isEnabled
}

func (p *AndroidSystemProxy) Enable() error {
	err := p.runAndroidShell("settings", "put", "global", "http_proxy", p.serverAddr.String())
	if err != nil {
		return err
	}
	p.isEnabled = true
	return nil
}

func (p *AndroidSystemProxy) Disable() error {
	err := p.runAndroidShell("settings", "put", "global", "http_proxy", ":0")
	if err != nil {
		return err
	}
	p.isEnabled = false
	return nil
}

func (p *AndroidSystemProxy) runAndroidShell(name string, args ...string) error {
	if !p.useRish {
		return shell.Exec(name, args...).Attach().Run()
	} else {
		return shell.Exec("sh", p.rishPath, "-c", F.ToString(name, " ", strings.Join(args, " "))).Attach().Run()
	}
}
