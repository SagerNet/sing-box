package settings

import (
	"context"
	"strconv"
	"strings"

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
	monitor       tun.DefaultInterfaceMonitor
	interfaceName string
	element       *list.Element[tun.DefaultInterfaceUpdateCallback]
	serverAddr    M.Socksaddr
	supportSOCKS  bool
	isEnabled     bool
}

func NewSystemProxy(ctx context.Context, serverAddr M.Socksaddr, supportSOCKS bool) (*DarwinSystemProxy, error) {
	interfaceMonitor := service.FromContext[adapter.NetworkManager](ctx).InterfaceMonitor()
	if interfaceMonitor == nil {
		return nil, E.New("missing interface monitor")
	}
	proxy := &DarwinSystemProxy{
		monitor:      interfaceMonitor,
		serverAddr:   serverAddr,
		supportSOCKS: supportSOCKS,
	}
	proxy.element = interfaceMonitor.RegisterCallback(proxy.routeUpdate)
	return proxy, nil
}

func (p *DarwinSystemProxy) IsEnabled() bool {
	return p.isEnabled
}

func (p *DarwinSystemProxy) Enable() error {
	return p.update0()
}

func (p *DarwinSystemProxy) Disable() error {
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
	if !p.isEnabled || defaultInterface == nil {
		return
	}
	_ = p.update0()
}

func (p *DarwinSystemProxy) update0() error {
	newInterface := p.monitor.DefaultInterface()
	if p.interfaceName == newInterface.Name {
		return nil
	}
	if p.interfaceName != "" {
		_ = p.Disable()
	}
	p.interfaceName = newInterface.Name
	interfaceDisplayName, err := getInterfaceDisplayName(p.interfaceName)
	if err != nil {
		return err
	}
	if p.supportSOCKS {
		err = shell.Exec("networksetup", "-setsocksfirewallproxy", interfaceDisplayName, p.serverAddr.AddrString(), strconv.Itoa(int(p.serverAddr.Port))).Attach().Run()
	}
	if err != nil {
		return err
	}
	err = shell.Exec("networksetup", "-setwebproxy", interfaceDisplayName, p.serverAddr.AddrString(), strconv.Itoa(int(p.serverAddr.Port))).Attach().Run()
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
	for _, deviceSpan := range strings.Split(string(content), "Ethernet Address") {
		if strings.Contains(deviceSpan, "Device: "+name) {
			substr := "Hardware Port: "
			deviceSpan = deviceSpan[strings.Index(deviceSpan, substr)+len(substr):]
			deviceSpan = deviceSpan[:strings.Index(deviceSpan, "\n")]
			return deviceSpan, nil
		}
	}
	return "", E.New(name, " not found in networksetup -listallhardwareports")
}
