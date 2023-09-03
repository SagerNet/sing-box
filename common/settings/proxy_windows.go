package settings

import (
	"context"

	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/wininet"
)

type WindowsSystemProxy struct {
	serverAddr   M.Socksaddr
	supportSOCKS bool
	isEnabled    bool
}

func NewSystemProxy(ctx context.Context, serverAddr M.Socksaddr, supportSOCKS bool) (*WindowsSystemProxy, error) {
	return &WindowsSystemProxy{
		serverAddr:   serverAddr,
		supportSOCKS: supportSOCKS,
	}, nil
}

func (p *WindowsSystemProxy) IsEnabled() bool {
	return p.isEnabled
}

func (p *WindowsSystemProxy) Enable() error {
	err := wininet.SetSystemProxy("http://"+p.serverAddr.String(), "")
	if err != nil {
		return err
	}
	p.isEnabled = true
	return nil
}

func (p *WindowsSystemProxy) Disable() error {
	err := wininet.ClearSystemProxy()
	if err != nil {
		return err
	}
	p.isEnabled = false
	return nil
}
