//go:build with_gvisor

package tailscale

import (
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	E "github.com/sagernet/sing/common/exceptions"
	tsTUN "github.com/sagernet/tailscale/net/tstun"
	"github.com/sagernet/tailscale/types/ipproto"
	"github.com/sagernet/tailscale/wgengine/filter"
)

func (t *Endpoint) PreMatchFlow(network string, destination netip.Addr) adapter.PreMatchAction {
	return adapter.PreMatchFlow
}

func (t *Endpoint) PortAddresses() (netip.Addr, netip.Addr) {
	if !t.started.Load() {
		return netip.Addr{}, netip.Addr{}
	}
	return t.server.TailscaleIPs()
}

func (t *Endpoint) PortMTU() uint32 {
	if t.systemInterface {
		return t.systemInterfaceMTU
	}
	return uint32(tsTUN.DefaultTUNMTU())
}

func (t *Endpoint) JudgeFlow(network uint8, source netip.AddrPort, destination netip.AddrPort, firstPacket []byte) tun.FlowVerdict {
	inet4Address, inet6Address := t.PortAddresses()
	if destination.Addr() == inet4Address || destination.Addr() == inet6Address {
		return tun.FlowVerdict{Action: tun.ActionAccept}
	}
	if t.filter != nil {
		tsFilter := t.filter.Load()
		if tsFilter != nil {
			var (
				ipProto         ipproto.Proto
				destinationPort uint16
			)
			switch network {
			case uint8(header.TCPProtocolNumber):
				ipProto = ipproto.TCP
				destinationPort = destination.Port()
			case uint8(header.UDPProtocolNumber):
				ipProto = ipproto.UDP
				destinationPort = destination.Port()
			case uint8(header.ICMPv4ProtocolNumber):
				ipProto = ipproto.ICMPv4
			case uint8(header.ICMPv6ProtocolNumber):
				ipProto = ipproto.ICMPv6
			}
			switch tsFilter.Check(source.Addr(), destination.Addr(), destinationPort, ipProto) {
			case filter.Drop:
				return tun.FlowVerdict{Action: tun.ActionReject}
			case filter.DropSilently:
				return tun.FlowVerdict{Action: tun.ActionDrop}
			}
		}
	}
	return adapter.JudgeFlow(t.router, t.Tag(), t.Type(), network, source, destination, firstPacket)
}

func (t *Endpoint) AttachReturn(returnPath tun.Return) error {
	t.returnAccess.Lock()
	defer t.returnAccess.Unlock()
	if t.returnPath == returnPath {
		return nil
	}
	if t.returnPath != nil {
		return E.New("return path already attached")
	}
	err := t.wgEngine.SetReturnPath(returnPath)
	if err != nil {
		return err
	}
	t.returnPath = returnPath
	return nil
}

func (t *Endpoint) DetachReturn(returnPath tun.Return) error {
	t.returnAccess.Lock()
	defer t.returnAccess.Unlock()
	if t.returnPath == returnPath {
		t.returnPath = nil
	}
	return nil
}

func (t *Endpoint) WritePackets(packets [][]byte) error {
	if !t.started.Load() {
		return E.New("Tailscale is not ready yet")
	}
	unmatched, err := t.wgEngine.InputPackets(packets)
	if err != nil || len(unmatched) == 0 {
		return err
	}
	t.returnAccess.Lock()
	returnPath := t.returnPath
	t.returnAccess.Unlock()
	if returnPath == nil {
		return nil
	}
	headroom := returnPath.ReturnHeadroom()
	inet4Address, inet6Address := t.PortAddresses()
	var replies [][]byte
	for _, packet := range unmatched {
		source := inet4Address
		if header.IPVersion(packet) == header.IPv6Version {
			source = inet6Address
		}
		reply, replyOk := tun.BuildUnreachable(packet, source, headroom)
		if replyOk {
			replies = append(replies, reply)
		}
	}
	if len(replies) > 0 {
		returnPath.ReturnPackets(replies)
	}
	return nil
}
