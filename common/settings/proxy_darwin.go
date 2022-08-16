package settings

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/x/list"
)

type systemProxy struct {
	monitor       tun.DefaultInterfaceMonitor
	interfaceName string
	element       *list.Element[tun.DefaultInterfaceUpdateCallback]
	port          uint16
	isMixed       bool
}

func (p *systemProxy) update() error {
	newInterfaceName := p.monitor.DefaultInterfaceName()
	if p.interfaceName == newInterfaceName {
		return nil
	}
	if p.interfaceName != "" {
		_ = p.unset()
	}
	p.interfaceName = newInterfaceName
	interfaceDisplayName, err := getInterfaceDisplayName(p.interfaceName)
	if err != nil {
		return err
	}
	if p.isMixed {
		err = common.Exec("networksetup", "-setsocksfirewallproxy", interfaceDisplayName, "127.0.0.1", F.ToString(p.port)).Attach().Run()
	}
	if err == nil {
		err = common.Exec("networksetup", "-setwebproxy", interfaceDisplayName, "127.0.0.1", F.ToString(p.port)).Attach().Run()
	}
	if err == nil {
		err = common.Exec("networksetup", "-setsecurewebproxy", interfaceDisplayName, "127.0.0.1", F.ToString(p.port)).Attach().Run()
	}
	return err
}

func (p *systemProxy) unset() error {
	interfaceDisplayName, err := getInterfaceDisplayName(p.interfaceName)
	if err != nil {
		return err
	}
	if p.isMixed {
		err = common.Exec("networksetup", "-setsocksfirewallproxystate", interfaceDisplayName, "off").Attach().Run()
	}
	if err == nil {
		err = common.Exec("networksetup", "-setwebproxystate", interfaceDisplayName, "off").Attach().Run()
	}
	if err == nil {
		err = common.Exec("networksetup", "-setsecurewebproxystate", interfaceDisplayName, "off").Attach().Run()
	}
	return err
}

func getInterfaceDisplayName(name string) (string, error) {
	content, err := common.Exec("networksetup", "-listallhardwareports").Read()
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
	err := proxy.update()
	if err != nil {
		return nil, err
	}
	proxy.element = interfaceMonitor.RegisterCallback(proxy.update)
	return func() error {
		interfaceMonitor.UnregisterCallback(proxy.element)
		return proxy.unset()
	}, nil
}
