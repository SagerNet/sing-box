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

func (p *DarwinSystemProxy) Enable() error {
	return p.update0()
}

func (p *DarwinSystemProxy) Disable() error {
	hardwarePorts, err := getMacOSActiveNetworkHardwarePorts()
	if err != nil {
		return err
	}
	for _, interfaceDisplayName := range hardwarePorts {
		if p.supportSOCKS {
			err = shell.Exec("networksetup", "-setsocksfirewallproxystate", interfaceDisplayName, "off").Attach().Run()
		}
		if err == nil {
			err = shell.Exec("networksetup", "-setwebproxystate", interfaceDisplayName, "off").Attach().Run()
		}
		if err == nil {
			err = shell.Exec("networksetup", "-setsecurewebproxystate", interfaceDisplayName, "off").Attach().Run()
		}
		if err != nil {
			return err
		}
	}
	p.isEnabled = false
	return nil
}

func (p *DarwinSystemProxy) update(event int) {
	if event&tun.EventInterfaceUpdate == 0 {
		return
	}
	if !p.isEnabled {
		return
	}
	_ = p.update0()
}

func (p *DarwinSystemProxy) update0() error {
	newInterfaceName := p.monitor.DefaultInterfaceName(netip.IPv4Unspecified())
	if p.interfaceName == newInterfaceName {
		return nil
	}
	if p.interfaceName != "" {
		_ = p.Disable()
	}
	p.interfaceName = newInterfaceName
	hardwarePorts, err := getMacOSActiveNetworkHardwarePorts()
	if err != nil {
		return err
	}
	for _, interfaceDisplayName := range hardwarePorts {
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
	}
	p.isEnabled = true
	return nil
}

func getMacOSActiveNetworkHardwarePorts() ([]string, error) {
	content, err := shell.Exec("networksetup", "-listallnetworkservices").ReadOutput()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	var hardwarePorts []string

	for _, line := range lines {
		if line == "An asterisk (*) denotes that a network service is disabled." {
			continue
		}
		if line == "" || strings.HasPrefix(line, "*") {
			continue
		}

		serviceContent, err := shell.Exec("networksetup", "-getinfo", line).ReadOutput()
		if err != nil {
			return nil, err
		}

		if strings.Contains(string(serviceContent), "IP address:") {
			hardwarePorts = append(hardwarePorts, line)
		}
	}

	if len(hardwarePorts) == 0 {
		return nil, E.New("Active Network Devices not found.")
	}
	return hardwarePorts, nil
}
