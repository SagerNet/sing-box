package wireguard

import (
	"net/netip"
	"sync/atomic"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/wireguard-go/device"
)

type peerRouter struct {
	memoryTun    *tun.MemoryTun
	inet4Address netip.Addr
	inet6Address netip.Addr
	routes       atomic.Pointer[peerRoutes]
}

type peerRoutes struct {
	allowedIPs *device.AllowedIPs
	queues     map[*device.Peer]*peerQueue
}

type peerQueue struct {
	peer    *device.Peer
	queue   *tun.OutboundQueue
	packets [][]byte
}

func newPeerRouter(options DeviceOptions) *peerRouter {
	router := &peerRouter{}
	router.inet4Address, router.inet6Address = deviceAddresses(options.Address)
	router.memoryTun = tun.NewMemoryTun(tun.MemoryTunOptions{
		MTU:      int(options.MTU),
		Outbound: router.writeUnrouted,
		Route:    router.route,
	})
	return router
}

func (r *peerRouter) setPeers(allowedIPs *device.AllowedIPs, peers []*device.Peer) {
	routes := &peerRoutes{
		allowedIPs: allowedIPs,
		queues:     make(map[*device.Peer]*peerQueue, len(peers)),
	}
	for _, peer := range peers {
		current := &peerQueue{peer: peer}
		current.queue = r.memoryTun.NewOutboundQueue(current.write)
		routes.queues[peer] = current
	}
	previous := r.routes.Swap(routes)
	if previous != nil {
		previous.close()
	}
}

func (r *peerRouter) route(packet []byte) *tun.OutboundQueue {
	routes := r.routes.Load()
	if routes == nil {
		return nil
	}
	var source, destination netip.Addr
	switch header.IPVersion(packet) {
	case header.IPv4Version:
		ipv4Header := header.IPv4(packet)
		source, destination = ipv4Header.SourceAddr(), ipv4Header.DestinationAddr()
	case header.IPv6Version:
		ipv6Header := header.IPv6(packet)
		source, destination = ipv6Header.SourceAddr(), ipv6Header.DestinationAddr()
	default:
		return nil
	}
	current := routes.queues[routes.allowedIPs.LookupFromPacket(source, destination, packet)]
	if current == nil {
		return nil
	}
	return current.queue
}

func (r *peerRouter) writeUnrouted(packetBuffers []*buf.Buffer) {
	defer buf.ReleaseMulti(packetBuffers)
	replies := make([][]byte, 0, len(packetBuffers))
	for _, packetBuffer := range packetBuffers {
		packet := packetBuffer.Bytes()
		source := r.inet4Address
		if header.IPVersion(packet) == header.IPv6Version {
			source = r.inet6Address
		}
		reply, built := tun.BuildICMPError(packet, tun.ICMPErrorNoRoute, source, 0, 0)
		if built {
			replies = append(replies, reply)
		}
	}
	if len(replies) > 0 {
		r.memoryTun.WritePackets(replies)
	}
}

func (r *peerRouter) close() error {
	routes := r.routes.Swap(nil)
	if routes != nil {
		routes.close()
	}
	return r.memoryTun.Close()
}

func (r *peerRoutes) close() {
	for _, current := range r.queues {
		current.queue.Close()
	}
}

func (q *peerQueue) write(packetBuffers []*buf.Buffer) {
	for _, packetBuffer := range packetBuffers {
		q.packets = append(q.packets, packetBuffer.Bytes())
	}
	q.peer.WritePackets(q.packets)
	clear(q.packets)
	q.packets = q.packets[:0]
	buf.ReleaseMulti(packetBuffers)
}
