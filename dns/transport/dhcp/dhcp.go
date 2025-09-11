package dhcp

import (
	"context"
	"errors"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
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
	"golang.org/x/exp/slices"
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
	transportLock     sync.RWMutex
	updatedAt         time.Time
	servers           []M.Socksaddr
	search            []string
	ndots             int
	attempts          int
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
		ndots:            1,
		attempts:         2,
	}, nil
}

func NewRawTransport(transportAdapter dns.TransportAdapter, ctx context.Context, dialer N.Dialer, logger log.ContextLogger) *Transport {
	return &Transport{
		TransportAdapter: transportAdapter,
		ctx:              ctx,
		dialer:           dialer,
		logger:           logger,
		networkManager:   service.FromContext[adapter.NetworkManager](ctx),
		ndots:            1,
		attempts:         2,
	}
}

func (t *Transport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if t.interfaceName == "" {
		t.interfaceCallback = t.networkManager.InterfaceMonitor().RegisterCallback(t.interfaceUpdated)
	}
	go func() {
		_, err := t.Fetch()
		if err != nil {
			t.logger.Error(E.Cause(err, "fetch DNS servers"))
		}
	}()
	return nil
}

func (t *Transport) Close() error {
	if t.interfaceCallback != nil {
		t.networkManager.InterfaceMonitor().UnregisterCallback(t.interfaceCallback)
	}
	return nil
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	servers, err := t.Fetch()
	if err != nil {
		return nil, err
	}
	if len(servers) == 0 {
		return nil, E.New("dhcp: empty DNS servers from response")
	}
	return t.Exchange0(ctx, message, servers)
}

func (t *Transport) Exchange0(ctx context.Context, message *mDNS.Msg, servers []M.Socksaddr) (*mDNS.Msg, error) {
	question := message.Question[0]
	domain := dns.FqdnToDomain(question.Name)
	if len(servers) == 1 || !(message.Question[0].Qtype == mDNS.TypeA || message.Question[0].Qtype == mDNS.TypeAAAA) {
		return t.exchangeSingleRequest(ctx, servers, message, domain)
	} else {
		return t.exchangeParallel(ctx, servers, message, domain)
	}
}

func (t *Transport) Fetch() ([]M.Socksaddr, error) {
	t.transportLock.RLock()
	updatedAt := t.updatedAt
	servers := t.servers
	t.transportLock.RUnlock()
	if time.Since(updatedAt) < C.DHCPTTL {
		return servers, nil
	}
	t.transportLock.Lock()
	defer t.transportLock.Unlock()
	if time.Since(t.updatedAt) < C.DHCPTTL {
		return t.servers, nil
	}
	err := t.updateServers()
	if err != nil {
		return nil, err
	}
	return t.servers, nil
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
	} else if len(t.servers) == 0 {
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
	var (
		packetConn net.PacketConn
		err        error
	)
	for i := 0; i < 5; i++ {
		packetConn, err = listener.ListenPacket(t.ctx, "udp4", listenAddr)
		if err == nil || !errors.Is(err, syscall.EADDRINUSE) {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return err
	}
	defer packetConn.Close()

	discovery, err := dhcpv4.NewDiscovery(iface.HardwareAddr, dhcpv4.WithBroadcast(true), dhcpv4.WithRequestedOptions(
		dhcpv4.OptionDomainName,
		dhcpv4.OptionDomainNameServer,
		dhcpv4.OptionDNSDomainSearchList,
	))
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
			if errors.Is(err, io.ErrShortBuffer) {
				continue
			}
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

		return t.recreateServers(iface, dhcpPacket)
	}
}

func (t *Transport) recreateServers(iface *control.Interface, dhcpPacket *dhcpv4.DHCPv4) error {
	searchList := dhcpPacket.DomainSearch()
	if searchList != nil && len(searchList.Labels) > 0 {
		t.search = searchList.Labels
	} else if dhcpPacket.DomainName() != "" {
		t.search = []string{dhcpPacket.DomainName()}
	}
	serverAddrs := common.Map(dhcpPacket.DNS(), func(it net.IP) M.Socksaddr {
		return M.SocksaddrFrom(M.AddrFromIP(it), 53)
	})
	if len(serverAddrs) > 0 && !slices.Equal(t.servers, serverAddrs) {
		t.logger.Info("dhcp: updated DNS servers from ", iface.Name, ": [", strings.Join(common.Map(serverAddrs, M.Socksaddr.String), ","), "], search: [", strings.Join(t.search, ","), "]")
	}
	t.servers = serverAddrs
	return nil
}
