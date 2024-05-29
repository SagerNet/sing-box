package settings

import (
	"context"
	"net/netip"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/shell"
	"github.com/sagernet/sing/common/x/list"
)

type DarwinSystemProxy struct {
	monitor       tun.DefaultInterfaceMonitor
	interfaceName string
	element       *list.Element[tun.DefaultInterfaceUpdateCallback]
	serverAddr    M.Socksaddr
	supportSOCKS  bool
	isEnabled     bool
}

func NewSystemProxy(ctx context.Context, serverAddr M.Socksaddr, supportSOCKS bool) (*DarwinSystemProxy, error) {
	interfaceMonitor := adapter.RouterFromContext(ctx).InterfaceMonitor()
	if interfaceMonitor == nil {
		return nil, E.New("missing interface monitor")
	}
	proxy := &DarwinSystemProxy{
		monitor:      interfaceMonitor,
		serverAddr:   serverAddr,
		supportSOCKS: supportSOCKS,
	}
	proxy.element = interfaceMonitor.RegisterCallback(proxy.update)
	return proxy, nil
}

func (p *DarwinSystemProxy) IsEnabled() bool {
	return p.isEnabled
}
