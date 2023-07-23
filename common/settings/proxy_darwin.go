package settings

import (
	"net/netip"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/shell"
	"github.com/sagernet/sing/common/x/list"
)

type systemProxy struct {
	monitor       tun.DefaultInterfaceMonitor
	interfaceName string
	element       *list.Element[tun.DefaultInterfaceUpdateCallback]
	port          uint16
	isMixed       bool
}

func (p *systemProxy) update(event int) {
	newInterfaceName := p.monitor.DefaultInterfaceName(netip.IPv4Unspecified())
	if p.interfaceName == newInterfaceName {
		return
	}
	if p.interfaceName != "" {
		_ = p.unset()
	}
	p.interfaceName = newInterfaceName
	interfaceDisplayName, err := getInterfaceDisplayName(p.interfaceName)
	if err != nil {
		return
	}
	if p.isMixed {
		err = shell.Exec("networksetup", "-setsocksfirewallproxy", interfaceDisplayName, "127.0.0.1", F.ToString(p.port)).Attach().Run()
	}
	if err == nil {
		err = shell.Exec("networksetup", "-setwebproxy", interfaceDisplayName, "127.0.0.1", F.ToString(p.port)).Attach().Run()
	}
	if err == nil {
		_ = shell.Exec("networksetup", "-setsecurewebproxy", interfaceDisplayName, "127.0.0.1", F.ToString(p.port)).Attach().Run()
	}
	return
}

func (p *systemProxy) unset() error {
	interfaceDisplayName, err := getInterfaceDisplayName(p.interfaceName)
	if err != nil {
		return err
	}
	if p.isMixed {
		err = shell.Exec("networksetup", "-setsocksfirewallproxystate", interfaceDisplayName, "off").Attach().Run()
	}
	if err == nil {
		err = shell.Exec("networksetup", "-setwebproxystate", interfaceDisplayName, "off").Attach().Run()
	}
	if err == nil {
		err = shell.Exec("networksetup", "-setsecurewebproxystate", interfaceDisplayName, "off").Attach().Run()
	}
	return err
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

func SetSystemProxy(router adapter.Router, port uint16, isMixed bool) (func() error, error) {
	interfaceMonitor := router.InterfaceMonitor()
	if interfaceMonitor == nil {
		return nil, E.New("missing interface monitor")
	}
	proxy := &systemProxy{
		monitor: interfaceMonitor,
		port:    port,
		isMixed: isMixed,
	}
	proxy.update(tun.EventInterfaceUpdate)
	proxy.element = interfaceMonitor.RegisterCallback(proxy.update)
	return func() error {
		interfaceMonitor.UnregisterCallback(proxy.element)
		return proxy.unset()
	}, nil
}
