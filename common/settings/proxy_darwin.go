package settings

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/shell"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
)

type DarwinSystemProxy struct {
	ctx           context.Context
	monitor       tun.DefaultInterfaceMonitor
	interfaceName string
	element       *list.Element[tun.DefaultInterfaceUpdateCallback]
	serverAddr    M.Socksaddr
	supportSOCKS  bool
	access        sync.Mutex
	updateCancel  context.CancelFunc
	isEnabled     bool
}

func NewSystemProxy(ctx context.Context, serverAddr M.Socksaddr, supportSOCKS bool, bypassDomain []string) (*DarwinSystemProxy, error) {
	interfaceMonitor := service.FromContext[adapter.NetworkManager](ctx).InterfaceMonitor()
	if interfaceMonitor == nil {
		return nil, E.New("missing interface monitor")
	}
	proxy := &DarwinSystemProxy{
		ctx:          ctx,
		monitor:      interfaceMonitor,
		serverAddr:   serverAddr,
		supportSOCKS: supportSOCKS,
	}
	proxy.element = interfaceMonitor.RegisterCallback(proxy.routeUpdate)
	return proxy, nil
}

func (p *DarwinSystemProxy) IsEnabled() bool {
	p.access.Lock()
	defer p.access.Unlock()
	return p.isEnabled
}

func (p *DarwinSystemProxy) Enable() error {
	p.access.Lock()
	defer p.access.Unlock()
	return p.updateLocked(p.ctx)
}

func (p *DarwinSystemProxy) Disable() error {
	p.access.Lock()
	defer p.access.Unlock()
	return p.disableLocked()
}

func (p *DarwinSystemProxy) Close() error {
	p.access.Lock()
	updateCancel := p.updateCancel
	p.updateCancel = nil
	p.access.Unlock()
	if updateCancel != nil {
		updateCancel()
	}
	p.monitor.UnregisterCallback(p.element)
	return nil
}

func (p *DarwinSystemProxy) disableLocked() error {
	interfaceDisplayName, err := getInterfaceDisplayName(p.interfaceName)
	if err != nil {
		return err
	}
	if p.supportSOCKS {
		err = shell.Exec("networksetup", "-setsocksfirewallproxystate", interfaceDisplayName, "off").Attach().Run()
	}
	if err == nil {
		err = shell.Exec("networksetup", "-setwebproxystate", interfaceDisplayName, "off").Attach().Run()
	}
	if err == nil {
		err = shell.Exec("networksetup", "-setsecurewebproxystate", interfaceDisplayName, "off").Attach().Run()
	}
	if err == nil {
		p.isEnabled = false
	}
	return err
}

func (p *DarwinSystemProxy) routeUpdate(defaultInterface *control.Interface, flags int) {
	if defaultInterface == nil {
		return
	}
	updateContext, updateCancel := context.WithCancel(p.ctx)
	p.access.Lock()
	previousCancel := p.updateCancel
	p.updateCancel = updateCancel
	p.access.Unlock()
	if previousCancel != nil {
		previousCancel()
	}
	go func() {
		defer updateCancel()
		p.access.Lock()
		defer p.access.Unlock()
		if !p.isEnabled || updateContext.Err() != nil {
			return
		}
		_ = p.updateLocked(updateContext)
	}()
}

func (p *DarwinSystemProxy) updateLocked(ctx context.Context) error {
	newInterface := p.monitor.DefaultInterface()
	if newInterface == nil || p.interfaceName == newInterface.Name {
		return nil
	}
	if p.interfaceName != "" {
		_ = p.disableLocked()
	}
	p.interfaceName = newInterface.Name
	interfaceDisplayName, err := getInterfaceDisplayName(p.interfaceName)
	if err != nil {
		return err
	}
	if p.supportSOCKS {
		err = ctx.Err()
		if err != nil {
			return err
		}
		err = shell.Exec("networksetup", "-setsocksfirewallproxy", interfaceDisplayName, p.serverAddr.AddrString(), strconv.Itoa(int(p.serverAddr.Port))).Attach().Run()
	}
	if err != nil {
		return err
	}
	err = ctx.Err()
	if err != nil {
		return err
	}
	err = shell.Exec("networksetup", "-setwebproxy", interfaceDisplayName, p.serverAddr.AddrString(), strconv.Itoa(int(p.serverAddr.Port))).Attach().Run()
	if err != nil {
		return err
	}
	err = ctx.Err()
	if err != nil {
		return err
	}
	err = shell.Exec("networksetup", "-setsecurewebproxy", interfaceDisplayName, p.serverAddr.AddrString(), strconv.Itoa(int(p.serverAddr.Port))).Attach().Run()
	if err != nil {
		return err
	}
	p.isEnabled = true
	return nil
}

func getInterfaceDisplayName(name string) (string, error) {
	content, err := shell.Exec("networksetup", "-listallhardwareports").ReadOutput()
	if err != nil {
		return "", err
	}
	for deviceSpan := range strings.SplitSeq(string(content), "Ethernet Address") {
		if strings.Contains(deviceSpan, "Device: "+name) {
			substr := "Hardware Port: "
			deviceSpan = deviceSpan[strings.Index(deviceSpan, substr)+len(substr):]
			deviceSpan = deviceSpan[:strings.Index(deviceSpan, "\n")]
			return deviceSpan, nil
		}
	}
	return "", E.New(name, " not found in networksetup -listallhardwareports")
}
