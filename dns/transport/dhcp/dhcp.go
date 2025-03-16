package dhcp

import (
	"context"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/task"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"

	"github.com/insomniacslk/dhcp/dhcpv4"
	mDNS "github.com/miekg/dns"
)

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.DHCPDNSServerOptions](registry, C.DNSTypeDHCP, NewTransport)
}

var _ adapter.DNSTransport = (*Transport)(nil)

type Transport struct {
	dns.TransportAdapter
	ctx               context.Context
	dialer            N.Dialer
	logger            logger.ContextLogger
	networkManager    adapter.NetworkManager
	interfaceName     string
	interfaceCallback *list.Element[tun.DefaultInterfaceUpdateCallback]
	transports        []adapter.DNSTransport
	updateAccess      sync.Mutex
	updatedAt         time.Time
}

func NewTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.DHCPDNSServerOptions) (adapter.DNSTransport, error) {
	transportDialer, err := dns.NewLocalDialer(ctx, options.LocalDNSServerOptions)
	if err != nil {
		return nil, err
	}
	return &Transport{
		TransportAdapter: dns.NewTransportAdapterWithLocalOptions(C.DNSTypeDHCP, tag, options.LocalDNSServerOptions),
		ctx:              ctx,
		dialer:           transportDialer,
		logger:           logger,
		networkManager:   service.FromContext[adapter.NetworkManager](ctx),
		interfaceName:    options.Interface,
	}, nil
}

func (t *Transport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := t.fetchServers()
	if err != nil {
		return err
	}
	if t.interfaceName == "" {
		t.interfaceCallback = t.networkManager.InterfaceMonitor().RegisterCallback(t.interfaceUpdated)
	}
	return nil
}

func (t *Transport) Close() error {
	for _, transport := range t.transports {
		transport.Reset()
	}
	if t.interfaceCallback != nil {
		t.networkManager.InterfaceMonitor().UnregisterCallback(t.interfaceCallback)
	}
	return nil
}

func (t *Transport) Reset() {
	for _, transport := range t.transports {
		transport.Reset()
	}
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	err := t.fetchServers()
	if err != nil {
		return nil, err
	}

	if len(t.transports) == 0 {
		return nil, E.New("dhcp: empty DNS servers from response")
	}

	var response *mDNS.Msg
	for _, transport := range t.transports {
		response, err = transport.Exchange(ctx, message)
		if err == nil {
			return response, nil
		}
	}
	return nil, err
}

func (t *Transport) fetchInterface() (*control.Interface, error) {
	if t.interfaceName == "" {
		if t.networkManager.InterfaceMonitor() == nil {
			return nil, E.New("missing monitor for auto DHCP, set route.auto_detect_interface")
		}
		defaultInterface := t.networkManager.InterfaceMonitor().DefaultInterface()
		if defaultInterface == nil {
			return nil, E.New("missing default interface")
		}
		return defaultInterface, nil
	} else {
		return t.networkManager.InterfaceFinder().ByName(t.interfaceName)
	}
}

func (t *Transport) fetchServers() error {
	if time.Since(t.updatedAt) < C.DHCPTTL {
		return nil
	}
	t.updateAccess.Lock()
	defer t.updateAccess.Unlock()
	if time.Since(t.updatedAt) < C.DHCPTTL {
		return nil
	}
	return t.updateServers()
}

func (t *Transport) updateServers() error {
	iface, err := t.fetchInterface()
	if err != nil {
		return E.Cause(err, "dhcp: prepare interface")
	}

	t.logger.Info("dhcp: query DNS servers on ", iface.Name)
	fetchCtx, cancel := context.WithTimeout(t.ctx, C.DHCPTimeout)
	err = t.fetchServers0(fetchCtx, iface)
	cancel()
	if err != nil {
		return err
	} else if len(t.transports) == 0 {
		return E.New("dhcp: empty DNS servers response")
	} else {
		t.updatedAt = time.Now()
		return nil
	}
}

func (t *Transport) interfaceUpdated(defaultInterface *control.Interface, flags int) {
	err := t.updateServers()
	if err != nil {
		t.logger.Error("update servers: ", err)
	}
}

func (t *Transport) fetchServers0(ctx context.Context, iface *control.Interface) error {
	var listener net.ListenConfig
	listener.Control = control.Append(listener.Control, control.BindToInterface(t.networkManager.InterfaceFinder(), iface.Name, iface.Index))
	listener.Control = control.Append(listener.Control, control.ReuseAddr())
	listenAddr := "0.0.0.0:68"
	if runtime.GOOS == "linux" || runtime.GOOS == "android" {
		listenAddr = "255.255.255.255:68"
	}
	packetConn, err := listener.ListenPacket(t.ctx, "udp4", listenAddr)
	if err != nil {
		return err
	}
	defer packetConn.Close()

	discovery, err := dhcpv4.NewDiscovery(iface.HardwareAddr, dhcpv4.WithBroadcast(true), dhcpv4.WithRequestedOptions(dhcpv4.OptionDomainNameServer))
	if err != nil {
		return err
	}

	_, err = packetConn.WriteTo(discovery.ToBytes(), &net.UDPAddr{IP: net.IPv4bcast, Port: 67})
	if err != nil {
		return err
	}

	var group task.Group
	group.Append0(func(ctx context.Context) error {
		return t.fetchServersResponse(iface, packetConn, discovery.TransactionID)
	})
	group.Cleanup(func() {
		packetConn.Close()
	})
	return group.Run(ctx)
}

func (t *Transport) fetchServersResponse(iface *control.Interface, packetConn net.PacketConn, transactionID dhcpv4.TransactionID) error {
	buffer := buf.NewSize(dhcpv4.MaxMessageSize)
	defer buffer.Release()

	for {
		_, _, err := buffer.ReadPacketFrom(packetConn)
		if err != nil {
			return err
		}

		dhcpPacket, err := dhcpv4.FromBytes(buffer.Bytes())
		if err != nil {
			t.logger.Trace("dhcp: parse DHCP response: ", err)
			return err
		}

		if dhcpPacket.MessageType() != dhcpv4.MessageTypeOffer {
			t.logger.Trace("dhcp: expected OFFER response, but got ", dhcpPacket.MessageType())
			continue
		}

		if dhcpPacket.TransactionID != transactionID {
			t.logger.Trace("dhcp: expected transaction ID ", transactionID, ", but got ", dhcpPacket.TransactionID)
			continue
		}

		dns := dhcpPacket.DNS()
		if len(dns) == 0 {
			return nil
		}
		return t.recreateServers(iface, common.Map(dns, func(it net.IP) M.Socksaddr {
			return M.SocksaddrFrom(M.AddrFromIP(it), 53)
		}))
	}
}

func (t *Transport) recreateServers(iface *control.Interface, serverAddrs []M.Socksaddr) error {
	if len(serverAddrs) > 0 {
		t.logger.Info("dhcp: updated DNS servers from ", iface.Name, ": [", strings.Join(common.Map(serverAddrs, M.Socksaddr.String), ","), "]")
	}
	serverDialer := common.Must1(dialer.NewDefault(t.ctx, option.DialerOptions{
		BindInterface:      iface.Name,
		UDPFragmentDefault: true,
	}))
	var transports []adapter.DNSTransport
	for _, serverAddr := range serverAddrs {
		transports = append(transports, transport.NewUDPRaw(t.logger, t.TransportAdapter, serverDialer, serverAddr))
	}
	for _, transport := range t.transports {
		transport.Reset()
	}
	t.transports = transports
	return nil
}
