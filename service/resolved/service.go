//go:build linux

package resolved

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/listener"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	dnsOutbound "github.com/sagernet/sing-box/protocol/dns"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"

	"github.com/godbus/dbus/v5"
	mDNS "github.com/miekg/dns"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.ResolvedServiceOptions](registry, C.TypeResolved, NewService)
}

type Service struct {
	boxService.Adapter
	ctx                   context.Context
	logger                log.ContextLogger
	network               adapter.NetworkManager
	dnsRouter             adapter.DNSRouter
	listener              *listener.Listener
	systemBus             *dbus.Conn
	linkAccess            sync.RWMutex
	links                 map[int32]*TransportLink
	defaultRouteSequence  []int32
	networkUpdateCallback *list.Element[tun.NetworkUpdateCallback]
	updateCallback        func(*TransportLink) error
	deleteCallback        func(*TransportLink)
}

type TransportLink struct {
	iif          *control.Interface
	address      []LinkDNS
	addressEx    []LinkDNSEx
	domain       []LinkDomain
	defaultRoute bool
	dnsOverTLS   bool
	// dnsOverTLSFallback bool
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.ResolvedServiceOptions) (adapter.Service, error) {
	inbound := &Service{
		Adapter:   boxService.NewAdapter(C.TypeResolved, tag),
		ctx:       ctx,
		logger:    logger,
		network:   service.FromContext[adapter.NetworkManager](ctx),
		dnsRouter: service.FromContext[adapter.DNSRouter](ctx),
		links:     make(map[int32]*TransportLink),
	}
	inbound.listener = listener.New(listener.Options{
		Context:                  ctx,
		Logger:                   logger,
		Network:                  []string{N.NetworkTCP, N.NetworkUDP},
		Listen:                   options.ListenOptions,
		ConnectionHandler:        inbound,
		OOBPacketHandler:         inbound,
		ThreadUnsafePacketWriter: true,
	})
	return inbound, nil
}

func (i *Service) Start(stage adapter.StartStage) error {
	switch stage {
	case adapter.StartStateInitialize:
		inboundManager := service.FromContext[adapter.ServiceManager](i.ctx)
		for _, transport := range inboundManager.Services() {
			if transport.Type() == C.TypeResolved && transport != i {
				return E.New("multiple resolved service are not supported")
			}
		}
		systemBus, err := dbus.SystemBus()
		if err != nil {
			return err
		}
		i.systemBus = systemBus
		err = systemBus.Export((*resolve1Manager)(i), "/org/freedesktop/resolve1", "org.freedesktop.resolve1.Manager")
		if err != nil {
			return err
		}
		reply, err := systemBus.RequestName("org.freedesktop.resolve1", dbus.NameFlagDoNotQueue)
		if err != nil {
			return err
		}
		switch reply {
		case dbus.RequestNameReplyPrimaryOwner:
		case dbus.RequestNameReplyExists:
			return E.New("D-Bus object already exists, maybe real resolved is running")
		default:
			return E.New("unknown request name reply: ", reply)
		}
		i.networkUpdateCallback = i.network.NetworkMonitor().RegisterCallback(i.onNetworkUpdate)
	case adapter.StartStateStart:
		err := i.listener.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *Service) Close() error {
	if i.networkUpdateCallback != nil {
		i.network.NetworkMonitor().UnregisterCallback(i.networkUpdateCallback)
	}
	if i.systemBus != nil {
		i.systemBus.ReleaseName("org.freedesktop.resolve1")
		i.systemBus.Close()
	}
	return i.listener.Close()
}

func (i *Service) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	metadata.Inbound = i.Tag()
	metadata.InboundType = i.Type()
	metadata.Destination = M.Socksaddr{}
	for {
		conn.SetReadDeadline(time.Now().Add(C.DNSTimeout))
		err := dnsOutbound.HandleStreamDNSRequest(ctx, i.dnsRouter, conn, metadata)
		if err != nil {
			N.CloseOnHandshakeFailure(conn, onClose, err)
			return
		}
	}
}

func (i *Service) NewPacketEx(buffer *buf.Buffer, oob []byte, source M.Socksaddr) {
	go i.exchangePacket(buffer, oob, source)
}

func (i *Service) exchangePacket(buffer *buf.Buffer, oob []byte, source M.Socksaddr) {
	ctx := log.ContextWithNewID(i.ctx)
	err := i.exchangePacket0(ctx, buffer, oob, source)
	if err != nil {
		i.logger.ErrorContext(ctx, "process DNS packet: ", err)
	}
}

func (i *Service) exchangePacket0(ctx context.Context, buffer *buf.Buffer, oob []byte, source M.Socksaddr) error {
	var message mDNS.Msg
	err := message.Unpack(buffer.Bytes())
	buffer.Release()
	if err != nil {
		return E.Cause(err, "unpack request")
	}
	var metadata adapter.InboundContext
	metadata.Source = source
	metadata.InboundType = i.Type()
	metadata.Inbound = i.Tag()
	response, err := i.dnsRouter.Exchange(adapter.WithContext(ctx, &metadata), &message, adapter.DNSQueryOptions{})
	if err != nil {
		return err
	}
	responseBuffer, err := dns.TruncateDNSMessage(&message, response, 0)
	if err != nil {
		return err
	}
	defer responseBuffer.Release()
	_, _, err = i.listener.UDPConn().WriteMsgUDPAddrPort(responseBuffer.Bytes(), oob, source.AddrPort())
	return err
}

func (i *Service) onNetworkUpdate() {
	i.linkAccess.Lock()
	defer i.linkAccess.Unlock()
	var deleteIfIndex []int
	for ifIndex, link := range i.links {
		iif, err := i.network.InterfaceFinder().ByIndex(int(ifIndex))
		if err != nil || iif != link.iif {
			deleteIfIndex = append(deleteIfIndex, int(ifIndex))
		}
		i.defaultRouteSequence = common.Filter(i.defaultRouteSequence, func(it int32) bool {
			return it != ifIndex
		})
		if i.deleteCallback != nil {
			i.deleteCallback(link)
		}
	}
	for _, ifIndex := range deleteIfIndex {
		delete(i.links, int32(ifIndex))
	}
}

func (conf *TransportLink) nameList(ndots int, name string) []string {
	search := common.Map(common.Filter(conf.domain, func(it LinkDomain) bool {
		return !it.RoutingOnly
	}), func(it LinkDomain) string {
		return it.Domain
	})

	l := len(name)
	rooted := l > 0 && name[l-1] == '.'
	if l > 254 || l == 254 && !rooted {
		return nil
	}

	if rooted {
		if avoidDNS(name) {
			return nil
		}
		return []string{name}
	}

	hasNdots := strings.Count(name, ".") >= ndots
	name += "."
	// l++

	names := make([]string, 0, 1+len(search))
	if hasNdots && !avoidDNS(name) {
		names = append(names, name)
	}
	for _, suffix := range search {
		fqdn := name + suffix
		if !avoidDNS(fqdn) && len(fqdn) <= 254 {
			names = append(names, fqdn)
		}
	}
	if !hasNdots && !avoidDNS(name) {
		names = append(names, name)
	}
	return names
}

func avoidDNS(name string) bool {
	if name == "" {
		return true
	}
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	return strings.HasSuffix(name, ".onion")
}
