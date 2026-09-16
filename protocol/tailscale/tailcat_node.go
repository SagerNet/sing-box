//go:build with_tailscale

package tailscale

import (
	"cmp"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/tailscale/disco"
	"github.com/sagernet/tailscale/health"
	tsDNS "github.com/sagernet/tailscale/net/dns"
	"github.com/sagernet/tailscale/net/dnscache"
	"github.com/sagernet/tailscale/net/netmon"
	"github.com/sagernet/tailscale/net/tsaddr"
	"github.com/sagernet/tailscale/net/tsdial"
	"github.com/sagernet/tailscale/tailcfg"
	"github.com/sagernet/tailscale/tsd"
	"github.com/sagernet/tailscale/types/ipproto"
	"github.com/sagernet/tailscale/types/key"
	tsLogger "github.com/sagernet/tailscale/types/logger"
	"github.com/sagernet/tailscale/types/netmap"
	"github.com/sagernet/tailscale/types/nettype"
	"github.com/sagernet/tailscale/types/views"
	"github.com/sagernet/tailscale/util/eventbus"
	"github.com/sagernet/tailscale/wgengine"
	"github.com/sagernet/tailscale/wgengine/filter"
	"github.com/sagernet/tailscale/wgengine/netstack"
	"github.com/sagernet/tailscale/wgengine/router"
	"github.com/sagernet/tailscale/wgengine/wgcfg"
	"github.com/sagernet/wireguard-go/device"

	"go4.org/netipx"
)

var (
	tailcatAllIPv6 = netip.MustParsePrefix("::/0")
	tailcatULASet  = sync.OnceValue(func() *netipx.IPSet {
		var builder netipx.IPSetBuilder
		builder.AddPrefix(tsaddr.TailscaleULARange())
		set, err := builder.IPSet()
		if err != nil {
			panic(err)
		}
		return set
	})
)

const (
	tailcatMeowTimeout         = 10 * time.Second
	tailcatMeowInterval        = time.Second
	tailcatMeowRefreshInterval = 30 * time.Second
)

type tailcatNodeOptions struct {
	Context        context.Context
	Logger         logger.ContextLogger
	PrivateKey     key.NodePrivate
	PresharedKey   device.NoisePresharedKey
	Region         *tailcfg.DERPRegion
	Hooks          netmon.Hooks
	LookupHook     dnscache.LookupHookFunc
	MemoryPressure func() tun.MemoryPressure

	// Server only
	Handler tun.Handler
	Users   map[key.NodePublic]string

	// Client only
	ServerPublicKey key.NodePublic
	ServerDiscoKey  key.DiscoPublic
}

type tailcatNode struct {
	ctx          context.Context
	cancel       context.CancelFunc
	logger       logger.ContextLogger
	logf         tsLogger.Logf
	sys          tsd.System
	privateKey   key.NodePrivate
	publicKey    key.NodePublic
	discoPrivate key.DiscoPrivate
	presharedKey device.NoisePresharedKey
	address      netip.Addr
	prefix       netip.Prefix
	region       *tailcfg.DERPRegion
	netstack     *netstack.Impl
	isServer     bool

	users           map[key.NodePublic]string
	serverPublicKey key.NodePublic
	serverDiscoKey  key.DiscoPublic
	sessionID       [16]byte
	meowed          chan struct{}
	meowResponses   chan [16]byte
	meowRequests    chan struct{}
	meowWait        sync.WaitGroup

	access           sync.Mutex
	clients          map[key.NodePublic]*tailcfg.Node
	clientsByAddress map[netip.Addr]key.NodePublic
	networkMap       *netmap.NetworkMap
	endpoints        []netip.AddrPort
	closeOnce        sync.Once
}

func newTailcatNode(options tailcatNodeOptions) (*tailcatNode, error) {
	publicKey := options.PrivateKey.Public()
	address := tailcatAddressForKey(publicKey)
	isServer := options.ServerPublicKey.IsZero()
	// Clients announce their disco key in the meow ping, so a fresh one
	// per tunnel makes a reconnecting client look new to the server,
	// which otherwise keeps trusting the previous direct path until it
	// times out. The server's must be derived: it is the key clients
	// are configured with.
	discoPrivate := key.NewDisco()
	if isServer {
		discoPrivate = tailcatDiscoPrivateForNode(options.PrivateKey)
	}
	ctx, cancel := context.WithCancel(options.Context)
	node := &tailcatNode{
		ctx:              ctx,
		cancel:           cancel,
		logger:           options.Logger,
		privateKey:       options.PrivateKey,
		publicKey:        publicKey,
		discoPrivate:     discoPrivate,
		presharedKey:     options.PresharedKey,
		address:          address,
		prefix:           netip.PrefixFrom(address, address.BitLen()),
		region:           options.Region,
		isServer:         isServer,
		users:            options.Users,
		serverPublicKey:  options.ServerPublicKey,
		serverDiscoKey:   options.ServerDiscoKey,
		meowed:           make(chan struct{}),
		meowResponses:    make(chan [16]byte, 1),
		meowRequests:     make(chan struct{}, 1),
		clientsByAddress: make(map[netip.Addr]key.NodePublic),
	}
	if isServer {
		rand.Read(node.sessionID[:])
	}
	node.logf = func(format string, args ...any) {
		node.logger.Trace(fmt.Sprintf(format, args...))
	}
	sys := &node.sys
	bus := eventbus.New()
	sys.Set(bus)
	sys.Set(health.NewTracker(bus))
	netMon, err := netmon.New(bus, node.logf, options.Hooks)
	if err != nil {
		node.close()
		return nil, E.Cause(err, "create network monitor")
	}
	sys.Set(netMon)
	dialer := &tsdial.Dialer{Logf: node.logf, Dialer: options.Hooks.Dialer}
	sys.Set(dialer)
	appName := "tailcat-client"
	if node.isServer {
		appName = "tailcat-server"
	}
	engine, err := wgengine.NewUserspaceEngine(node.logf, wgengine.Config{
		Context:       ctx,
		NetMon:        netMon,
		Dialer:        dialer,
		SetSubsystem:  sys.Set,
		Metrics:       sys.UserMetricsRegistry(),
		HealthTracker: sys.HealthTracker.Get(),
		EventBus:      bus,
		OnDERPRecv:    node.onDERPRecv,
		DERPAppName:   appName,
		ForceDiscoKey: node.discoPrivate,
		LookupHook:    options.LookupHook,
	})
	if err != nil {
		node.close()
		return nil, E.Cause(err, "create WireGuard engine")
	}
	sys.Set(engine)
	sys.NetstackRouter.Set(true)
	netStack, err := netstack.Create(node.logf, sys.Tun.Get(), sys.DNSManager.Get(), sys.ProxyMapper(), options.MemoryPressure)
	if err != nil {
		node.close()
		return nil, E.Cause(err, "create netstack")
	}
	netStack.ProcessLocalIPs = true
	if node.isServer {
		netStack.ProcessSubnets = true
		netStack.Handler = options.Handler
	} else {
		netStack.GetTCPHandlerForFlow = func(source, destination netip.AddrPort) (func(net.Conn), bool) {
			return nil, true
		}
		netStack.GetUDPHandlerForFlow = func(source, destination netip.AddrPort) (func(nettype.ConnPacketConn), bool) {
			return nil, true
		}
	}
	node.netstack = netStack
	sys.Set(netStack)
	sys.Tun.Get().Start()
	engine.SetFilter(node.buildFilter())
	return node, nil
}

func (n *tailcatNode) buildFilter() *filter.Filter {
	var localNets netipx.IPSetBuilder
	localNets.AddPrefix(n.prefix)
	var matches []filter.Match
	if n.isServer {
		localNets.AddPrefix(tailcatAllIPv6)
		matches = []filter.Match{{
			IPProto: views.SliceOf([]ipproto.Proto{ipproto.TCP, ipproto.UDP}),
			Srcs:    []netip.Prefix{tailcatAllIPv6},
			Dsts:    []filter.NetPortRange{{Net: tailcatAllIPv6, Ports: filter.PortRange{First: 0, Last: 65535}}},
		}}
	}
	local, err := localNets.IPSet()
	if err != nil {
		panic(err)
	}
	return filter.New(matches, nil, local, tailcatULASet(), nil, n.logf)
}

func (n *tailcatNode) start() error {
	err := n.netstack.Start(nil)
	if err != nil {
		return E.Cause(err, "start netstack")
	}
	engine := n.sys.Engine.Get()
	magicConn := n.sys.MagicSock.Get()
	err = magicConn.SetPrivateKey(n.privateKey)
	if err != nil {
		return E.Cause(err, "set private key")
	}
	magicConn.SetDERPMap(&tailcfg.DERPMap{Regions: map[int]*tailcfg.DERPRegion{n.region.RegionID: n.region}})
	networkMap := n.buildNetworkMap(nil)
	n.access.Lock()
	n.networkMap = networkMap
	n.access.Unlock()
	magicConn.SetNetworkMap(networkMap.SelfNode, networkMap.Peers)
	engine.SetSelfNode(networkMap.SelfNode)
	n.netstack.UpdateNetstackIPs(networkMap)
	magicConn.SetNetworkUp(true)
	engine.SetPeerConfigFunc(n.peerConfig)
	engine.SetPeerByIPPacketFunc(n.peerByIP)
	engine.SetPeerForIPFunc(n.peerForIP)
	engine.SetStatusCallback(n.onEngineStatus)
	wgConfig := &wgcfg.Config{
		PrivateKey: n.privateKey,
		Addresses:  []netip.Prefix{n.prefix},
	}
	if n.isServer {
		wgConfig.Addresses = append(wgConfig.Addresses, tailcatAllIPv6)
	}
	err = engine.Reconfig(wgConfig, &router.Config{LocalAddrs: []netip.Prefix{n.prefix}}, &tsDNS.Config{})
	if err != nil {
		return E.Cause(err, "configure WireGuard engine")
	}
	n.sys.NetMon.Get().Start()
	if !n.isServer {
		n.meowWait.Go(n.loopMeow)
	}
	return nil
}

func (n *tailcatNode) close() {
	n.closeOnce.Do(func() {
		n.cancel()
		n.meowWait.Wait()
		if n.netstack != nil {
			n.netstack.Close()
		}
		if engine, loaded := n.sys.Engine.GetOK(); loaded {
			engine.Close()
		}
		if netMon, loaded := n.sys.NetMon.GetOK(); loaded {
			netMon.Close()
		}
		if dialer, loaded := n.sys.Dialer.GetOK(); loaded {
			dialer.Close()
		}
		if bus, loaded := n.sys.Bus.GetOK(); loaded {
			bus.Close()
		}
	})
}

func (n *tailcatNode) selfNode() *tailcfg.Node {
	node := &tailcfg.Node{
		ID:        1,
		StableID:  "1",
		Name:      "server.tailcat.",
		User:      100,
		Key:       n.publicKey,
		DiscoKey:  n.discoPrivate.Public(),
		Addresses: []netip.Prefix{n.prefix},
		HomeDERP:  n.region.RegionID,
	}
	if n.isServer {
		node.AllowedIPs = []netip.Prefix{n.prefix, tailcatAllIPv6}
	} else {
		node.ID = 2
		node.StableID = "2"
		node.Name = "client.tailcat."
	}
	return node
}

// buildNetworkMap must be called with n.access held when clients is
// the live map; start passes nil before any client exists.
func (n *tailcatNode) buildNetworkMap(clients map[key.NodePublic]*tailcfg.Node) *netmap.NetworkMap {
	networkMap := &netmap.NetworkMap{
		NodeKey:  n.publicKey,
		SelfNode: n.selfNode().View(),
	}
	if n.isServer {
		for _, client := range clients {
			networkMap.Peers = append(networkMap.Peers, client.View())
		}
		slices.SortFunc(networkMap.Peers, func(a, b tailcfg.NodeView) int {
			return cmp.Compare(a.ID(), b.ID())
		})
	} else {
		networkMap.Peers = []tailcfg.NodeView{(&tailcfg.Node{
			ID:         1,
			StableID:   "1",
			Name:       "server.tailcat.",
			User:       100,
			Key:        n.serverPublicKey,
			DiscoKey:   n.serverDiscoKey,
			Addresses:  []netip.Prefix{tailcatPrefixForKey(n.serverPublicKey)},
			AllowedIPs: []netip.Prefix{tailcatPrefixForKey(n.serverPublicKey), tailcatAllIPv6},
			HomeDERP:   n.region.RegionID,
		}).View()}
	}
	return networkMap
}

func (n *tailcatNode) peerConfig(nodeKey key.NodePublic) (wgcfg.PeerConfig, bool) {
	if !n.isServer {
		if nodeKey != n.serverPublicKey {
			return wgcfg.PeerConfig{}, false
		}
		return wgcfg.PeerConfig{
			AllowedIPs:   []netip.Prefix{tailcatPrefixForKey(nodeKey), tailcatAllIPv6},
			PresharedKey: n.presharedKey,
		}, true
	}
	n.access.Lock()
	defer n.access.Unlock()
	client, loaded := n.clients[nodeKey]
	if !loaded {
		return wgcfg.PeerConfig{}, false
	}
	return wgcfg.PeerConfig{AllowedIPs: client.AllowedIPs, PresharedKey: n.presharedKey}, true
}

func (n *tailcatNode) peerByIP(destination netip.Addr) (key.NodePublic, bool) {
	if !n.isServer {
		return n.serverPublicKey, true
	}
	n.access.Lock()
	defer n.access.Unlock()
	nodeKey, loaded := n.clientsByAddress[destination]
	return nodeKey, loaded
}

func (n *tailcatNode) peerForIP(address netip.Addr) (wgengine.PeerForIP, bool) {
	n.access.Lock()
	defer n.access.Unlock()
	if n.networkMap == nil {
		return wgengine.PeerForIP{}, false
	}
	if tailcatNodeHasAddress(n.networkMap.SelfNode, address) {
		return wgengine.PeerForIP{Node: n.networkMap.SelfNode, IsSelf: true}, true
	}
	for _, peer := range n.networkMap.Peers {
		if tailcatNodeHasAddress(peer, address) {
			return wgengine.PeerForIP{Node: peer}, true
		}
	}
	return wgengine.PeerForIP{}, false
}

func tailcatNodeHasAddress(node tailcfg.NodeView, address netip.Addr) bool {
	if !node.Valid() {
		return false
	}
	for _, prefix := range node.Addresses().All() {
		if prefix.Addr() == address {
			return true
		}
	}
	return false
}

func (n *tailcatNode) userName(address netip.Addr) string {
	n.access.Lock()
	defer n.access.Unlock()
	nodeKey, loaded := n.clientsByAddress[address]
	if !loaded {
		return ""
	}
	return n.users[nodeKey]
}

func (n *tailcatNode) onDERPRecv(regionID int, source key.NodePublic, packet []byte) bool {
	if !isTailcatMeowPacket(packet) {
		return false
	}
	if isTailcatMeowedPacket(packet) {
		if !n.isServer && source == n.serverPublicKey {
			var sessionID [16]byte
			if len(packet) >= 5+len(sessionID) {
				copy(sessionID[:], packet[5:5+len(sessionID)])
			}
			select {
			case n.meowResponses <- sessionID:
			default:
			}
		}
		return true
	}
	if !n.isServer {
		return true
	}
	nodeKey, discoKey, valid := parseTailcatMeowPing(packet)
	if !valid {
		return false
	}
	if nodeKey.IsZero() || nodeKey != source {
		return true
	}
	go func() {
		if !n.onMeow(nodeKey, discoKey) {
			return
		}
		_, err := n.sys.MagicSock.Get().SendDERPPacketTo(source, regionID, encodeTailcatMeowed(n.sessionID))
		if err != nil {
			n.logger.Debug(E.Cause(err, "reply meow from ", nodeKey.ShortString()))
		}
	}()
	return true
}

func (n *tailcatNode) onMeow(nodeKey key.NodePublic, discoKey key.DiscoPublic) bool {
	n.access.Lock()
	defer n.access.Unlock()
	if n.users != nil {
		if _, allowed := n.users[nodeKey]; !allowed {
			n.logger.Debug("ignored meow from unknown client ", nodeKey.ShortString())
			return false
		}
	}
	previous, loaded := n.clients[nodeKey]
	if loaded && previous.DiscoKey == discoKey {
		return true
	}
	id := tailcfg.NodeID(len(n.clients) + 2)
	if loaded {
		n.logger.Info("client ", nodeKey.ShortString(), " reconnected")
		id = previous.ID
	} else {
		n.logger.Info("client ", nodeKey.ShortString(), " connected")
	}
	address := tailcatAddressForKey(nodeKey)
	prefix := netip.PrefixFrom(address, address.BitLen())
	if n.clients == nil {
		n.clients = make(map[key.NodePublic]*tailcfg.Node)
	}
	// A reconnecting client presents a new disco key, and its node is
	// replaced rather than updated in place: magicsock skips a netmap
	// update whose peer views all compare equal, and a view shares the
	// node it wraps, so mutation would be invisible to it.
	n.clients[nodeKey] = &tailcfg.Node{
		ID:         id,
		StableID:   tailcfg.StableNodeID(fmt.Sprint(id)),
		Name:       fmt.Sprintf("client%d.tailcat.", id),
		User:       100,
		Key:        nodeKey,
		DiscoKey:   discoKey,
		Addresses:  []netip.Prefix{prefix},
		AllowedIPs: []netip.Prefix{prefix},
		HomeDERP:   n.region.RegionID,
	}
	n.clientsByAddress[address] = nodeKey
	networkMap := n.buildNetworkMap(n.clients)
	n.networkMap = networkMap
	n.sys.MagicSock.Get().SetNetworkMap(networkMap.SelfNode, networkMap.Peers)
	if loaded {
		n.sys.Engine.Get().ResetDevicePeer(nodeKey)
	}
	n.netstack.UpdateNetstackIPs(networkMap)
	go n.advertiseEndpoints()
	return true
}

func (n *tailcatNode) onEngineStatus(status *wgengine.Status, err error) {
	if err != nil || status == nil {
		return
	}
	endpoints := make([]netip.AddrPort, 0, len(status.LocalAddrs))
	for _, endpoint := range status.LocalAddrs {
		endpoints = append(endpoints, endpoint.Addr)
	}
	slices.SortFunc(endpoints, func(a, b netip.AddrPort) int { return a.Compare(b) })
	endpoints = slices.Compact(endpoints)
	n.access.Lock()
	changed := !slices.Equal(endpoints, n.endpoints)
	if changed {
		n.endpoints = endpoints
	}
	n.access.Unlock()
	if changed && len(endpoints) > 0 {
		go n.advertiseEndpoints()
		if !n.isServer {
			n.requestMeow()
		}
	}
}

// Without a control plane distributing endpoints, magicsock never
// tries a direct path to a peer; a call-me-maybe over DERP, sealed
// exactly as magicsock's own sendDiscoMessage does, is what makes the
// peer disco-ping our endpoints.
func (n *tailcatNode) advertiseEndpoints() {
	n.access.Lock()
	endpoints := slices.Clone(n.endpoints)
	var peers []tailcfg.NodeView
	if n.networkMap != nil {
		peers = n.networkMap.Peers
	}
	n.access.Unlock()
	if len(endpoints) == 0 || len(peers) == 0 {
		return
	}
	payload := (&disco.CallMeMaybe{MyNumber: endpoints}).AppendMarshal(nil)
	magicConn := n.sys.MagicSock.Get()
	for _, peer := range peers {
		packet := make([]byte, 0, 512)
		packet = append(packet, disco.Magic...)
		packet = n.discoPrivate.Public().AppendTo(packet)
		packet = append(packet, n.discoPrivate.Shared(peer.DiscoKey()).Seal(payload)...)
		_, err := magicConn.SendDERPPacketTo(peer.Key(), n.region.RegionID, packet)
		if err != nil {
			n.logger.Debug(E.Cause(err, "advertise endpoints to ", peer.Key().ShortString()))
		}
	}
}

func (n *tailcatNode) meow(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, tailcatMeowTimeout)
	defer cancel()
	select {
	case <-n.meowed:
		return nil
	case <-ctx.Done():
		return E.Cause(ctx.Err(), "wait for server")
	case <-n.ctx.Done():
		return net.ErrClosed
	}
}

func (n *tailcatNode) requestMeow() {
	select {
	case n.meowRequests <- struct{}{}:
	default:
	}
}

func (n *tailcatNode) loopMeow() {
	magicConn := n.sys.MagicSock.Get()
	packet := encodeTailcatMeowPing(n.publicKey, n.discoPrivate.Public())
	var registered bool
	var serverSession [16]byte
	var lastSent time.Time
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-n.ctx.Done():
			return
		case sessionID := <-n.meowResponses:
			restarted := registered && serverSession != sessionID
			if restarted {
				n.sys.Engine.Get().ResetDevicePeer(n.serverPublicKey)
				n.logger.Info("server reconnected")
			}
			serverSession = sessionID
			if !registered || restarted {
				go n.advertiseEndpoints()
			}
			if !registered {
				registered = true
				close(n.meowed)
			}
			timer.Reset(tailcatMeowRefreshInterval)
		case <-n.meowRequests:
			timer.Reset(max(0, tailcatMeowInterval-time.Since(lastSent)))
		case <-timer.C:
			_, err := magicConn.SendDERPPacketTo(n.serverPublicKey, n.region.RegionID, packet)
			if err != nil {
				n.logger.Debug(E.Cause(err, "send meow"))
			}
			lastSent = time.Now()
			timer.Reset(tailcatMeowInterval)
		}
	}
}

func (n *tailcatNode) stack() *tun.Go {
	return n.netstack.ExportIPStack()
}

func (n *tailcatNode) interfaceUpdated() {
	if netMon, loaded := n.sys.NetMon.GetOK(); loaded {
		netMon.InjectEvent()
	}
	if !n.isServer {
		n.requestMeow()
	}
}
