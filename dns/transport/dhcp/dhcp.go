package dhcp

import (
	"context"
	"errors"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
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
	"golang.org/x/exp/slices"
)

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.DHCPDNSServerOptions](registry, C.DNSTypeDHCP, NewTransport)
}

var _ adapter.DNSTransport = (*Transport)(nil)

var errInterfaceIsCellular = E.New("interface is cellular")

type Transport struct {
	dns.TransportAdapter
	ctx               context.Context
	dialer            N.Dialer
	logger            logger.ContextLogger
	networkManager    adapter.NetworkManager
	platformInterface adapter.PlatformInterface
	interfaceName     string
	interfaceCallback *list.Element[tun.DefaultInterfaceUpdateCallback]
	transportLock     sync.RWMutex
	updatedAt         time.Time
	lastError         error
	servers           []M.Socksaddr
	serverTransports  []adapter.DNSTransport
	refreshing        atomic.Bool
	search            []string
	ndots             int
	attempts          int
	optional          bool
}

func NewTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.DHCPDNSServerOptions) (adapter.DNSTransport, error) {
	transportDialer, err := dns.NewLocalDialer(ctx, options.LocalDNSServerOptions)
	if err != nil {
		return nil, err
	}
	return &Transport{
		TransportAdapter:  dns.NewTransportAdapterWithLocalOptions(C.DNSTypeDHCP, tag, options.LocalDNSServerOptions),
		ctx:               ctx,
		dialer:            transportDialer,
		logger:            logger,
		networkManager:    service.FromContext[adapter.NetworkManager](ctx),
		platformInterface: service.FromContext[adapter.PlatformInterface](ctx),
		interfaceName:     options.Interface,
		ndots:             1,
		attempts:          2,
	}, nil
}

func NewRawTransport(transportAdapter dns.TransportAdapter, ctx context.Context, dialer N.Dialer, logger log.ContextLogger) *Transport {
	return &Transport{
		TransportAdapter:  transportAdapter,
		ctx:               ctx,
		dialer:            dialer,
		logger:            logger,
		networkManager:    service.FromContext[adapter.NetworkManager](ctx),
		platformInterface: service.FromContext[adapter.PlatformInterface](ctx),
		ndots:             1,
		attempts:          2,
		optional:          true,
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
		err := t.fetch()
		if err != nil {
			if errors.Is(err, errInterfaceIsCellular) && t.optional {
				t.logger.Debug(E.Cause(errInterfaceIsCellular, "dhcp: fetch DNS servers"))
			} else {
				t.logger.Error(E.Cause(err, "dhcp: fetch DNS servers"))
			}
		}
	}()
	return nil
}

func (t *Transport) Close() error {
	if t.interfaceCallback != nil {
		t.networkManager.InterfaceMonitor().UnregisterCallback(t.interfaceCallback)
	}
	t.transportLock.Lock()
	defer t.transportLock.Unlock()
	t.closeServerTransports()
	return nil
}

func (t *Transport) Reset() {
	t.transportLock.Lock()
	t.updatedAt = time.Time{}
	t.lastError = nil
	t.servers = nil
	t.closeServerTransports()
	t.transportLock.Unlock()
}

func (t *Transport) closeServerTransports() {
	for _, serverTransport := range t.serverTransports {
		serverTransport.Close()
	}
	t.serverTransports = nil
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	done := make(chan struct{})
	var (
		response *mDNS.Msg
		err      error
	)
	t.ExchangeAsync(ctx, message, func(callbackResponse *mDNS.Msg, callbackErr error) {
		response = callbackResponse
		err = callbackErr
		close(done)
	})
	<-done
	return response, err
}

func (t *Transport) ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	t.transportLock.RLock()
	updatedAt := t.updatedAt
	lastError := t.lastError
	serverTransports := t.serverTransports
	t.transportLock.RUnlock()
	if lastError != nil {
		callback(nil, E.Cause(lastError, "dhcp: fetch DNS servers"))
		return
	}
	if len(serverTransports) == 0 {
		go t.exchangeCold(ctx, message, callback)
		return
	}
	if time.Since(updatedAt) >= C.DHCPTTL {
		t.startRefresh()
	}
	t.exchangeWithTransports(ctx, message, serverTransports, callback)
}

func (t *Transport) exchangeCold(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	err := t.fetch()
	if err != nil {
		callback(nil, E.Cause(err, "dhcp: fetch DNS servers"))
		return
	}
	t.transportLock.RLock()
	serverTransports := t.serverTransports
	t.transportLock.RUnlock()
	if len(serverTransports) == 0 {
		callback(nil, E.New("dhcp: empty DNS servers from response"))
		return
	}
	t.exchangeWithTransports(ctx, message, serverTransports, callback)
}

func (t *Transport) Fetch() []M.Socksaddr {
	t.transportLock.RLock()
	updatedAt := t.updatedAt
	lastError := t.lastError
	servers := t.servers
	t.transportLock.RUnlock()
	if lastError != nil {
		return nil
	}
	if len(servers) > 0 && time.Since(updatedAt) >= C.DHCPTTL {
		t.startRefresh()
	}
	return servers
}

func (t *Transport) fetch() error {
	t.transportLock.RLock()
	updatedAt := t.updatedAt
	lastError := t.lastError
	t.transportLock.RUnlock()
	if lastError != nil {
		return lastError
	}
	if time.Since(updatedAt) < C.DHCPTTL {
		return nil
	}
	t.transportLock.Lock()
	defer t.transportLock.Unlock()
	if time.Since(t.updatedAt) < C.DHCPTTL {
		return nil
	}
	return t.updateServers()
}

func (t *Transport) startRefresh() {
	if !t.refreshing.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer t.refreshing.Store(false)
		t.transportLock.Lock()
		defer t.transportLock.Unlock()
		if time.Since(t.updatedAt) < C.DHCPTTL {
			return
		}
		err := t.updateServers()
		if err != nil {
			if errors.Is(err, errInterfaceIsCellular) && t.optional {
				t.logger.Debug(E.Cause(err, "dhcp: refresh DNS servers"))
			} else {
				t.logger.Error(E.Cause(err, "dhcp: refresh DNS servers"))
			}
		}
	}()
}

func (t *Transport) fetchInterface() (*control.Interface, error) {
	if t.interfaceName == "" {
		if t.networkManager.InterfaceMonitor() == nil {
			return nil, E.New("missing monitor for auto DHCP, set route.auto_detect_interface")
		}
		if t.platformInterface != nil && t.platformInterface.UsePlatformNetworkInterfaces() {
			defaultInterface := t.networkManager.DefaultNetworkInterface()
			if defaultInterface == nil {
				return nil, E.New("missing default interface")
			}
			if defaultInterface.Type == C.InterfaceTypeCellular {
				return nil, errInterfaceIsCellular
			}
			return &defaultInterface.Interface, nil
		} else {
			defaultInterface := t.networkManager.InterfaceMonitor().DefaultInterface()
			if defaultInterface == nil {
				return nil, E.New("missing default interface")
			}
			return defaultInterface, nil
		}
	} else {
		return t.networkManager.InterfaceFinder().ByName(t.interfaceName)
	}
}

func (t *Transport) updateServers() error {
	iface, err := t.fetchInterface()
	if err != nil {
		t.lastError = err
		t.updatedAt = time.Now()
		return E.Cause(err, "prepare interface")
	}
	t.logger.Info("dhcp: query DNS servers on ", iface.Name)
	fetchCtx, cancel := context.WithTimeout(t.ctx, C.DHCPTimeout)
	err = t.fetchServers0(fetchCtx, iface)
	cancel()
	t.updatedAt = time.Now()
	if err != nil {
		t.lastError = err
		return err
	} else if len(t.servers) == 0 {
		t.lastError = E.New("dhcp: empty DNS servers response")
		return t.lastError
	} else {
		t.lastError = nil
		return nil
	}
}

func (t *Transport) interfaceUpdated(defaultInterface *control.Interface, flags int) {
	t.transportLock.Lock()
	err := t.updateServers()
	t.transportLock.Unlock()
	if err != nil {
		if errors.Is(err, errInterfaceIsCellular) && t.optional {
			t.logger.Debug(E.Cause(errInterfaceIsCellular, "dhcp: update DNS servers"))
		} else {
			t.logger.Error("dhcp: update DNS servers: ", err)
		}
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
	for range 5 {
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
		buffer.Reset()
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
		t.search = common.Filter(common.Map(searchList.Labels, mDNS.Fqdn), func(it string) bool {
			return it != "."
		})
	} else if dhcpPacket.DomainName() != "" {
		domainName := mDNS.Fqdn(dhcpPacket.DomainName())
		if domainName != "." {
			t.search = []string{domainName}
		}
	}
	serverAddrs := common.Map(dhcpPacket.DNS(), func(it net.IP) M.Socksaddr {
		return M.SocksaddrFrom(M.AddrFromIP(it), 53)
	})
	if len(serverAddrs) > 0 && !slices.Equal(t.servers, serverAddrs) {
		t.logger.Info("dhcp: updated DNS servers from ", iface.Name, ": [", strings.Join(common.Map(serverAddrs, M.Socksaddr.String), ","), "], search: [", strings.Join(t.search, ","), "]")
	}
	if !slices.Equal(t.servers, serverAddrs) || t.serverTransports == nil {
		t.closeServerTransports()
		serverTransports := make([]adapter.DNSTransport, 0, len(serverAddrs))
		for _, serverAddr := range serverAddrs {
			serverTransport := transport.NewUDPRaw(t.logger, dns.NewTransportAdapter(C.DNSTypeUDP, "", nil), t.dialer, serverAddr)
			err := serverTransport.Start(adapter.StartStateStart)
			if err != nil {
				for _, startedTransport := range serverTransports {
					startedTransport.Close()
				}
				return E.Cause(err, "initialize transport for ", serverAddr)
			}
			serverTransports = append(serverTransports, serverTransport)
		}
		t.serverTransports = serverTransports
	}
	t.servers = serverAddrs
	return nil
}
