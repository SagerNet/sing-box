package libbox

import (
	"net/netip"
	"syscall"

	"github.com/sagernet/sing-box/experimental/libbox/internal/procfs"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

func FindConnectionOwner(ipProtocol int32, sourceAddress string, sourcePort int32, destinationAddress string, destinationPort int32) (*ConnectionOwner, error) {
	var network string
	switch ipProtocol {
	case syscall.IPPROTO_TCP:
		network = N.NetworkTCP
	case syscall.IPPROTO_UDP:
		network = N.NetworkUDP
	default:
		return nil, E.New("unknown protocol: ", ipProtocol)
	}
	sourceIP, err := netip.ParseAddr(sourceAddress)
	if err != nil {
		return nil, E.Cause(err, "parse source")
	}
	destinationIP, _ := netip.ParseAddr(destinationAddress)
	uid := procfs.ResolveSocketByProcSearch(network, netip.AddrPortFrom(sourceIP.Unmap(), uint16(sourcePort)), netip.AddrPortFrom(destinationIP.Unmap(), uint16(destinationPort)))
	if uid == -1 {
		return nil, E.New("procfs: not found")
	}
	return &ConnectionOwner{UserId: uid}, nil
}
