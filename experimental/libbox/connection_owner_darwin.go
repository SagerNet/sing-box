package libbox

import (
	"net/netip"
	"os/user"
	"syscall"

	"github.com/sagernet/sing-box/common/process"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

func FindConnectionOwner(ipProtocol int32, sourceAddress string, sourcePort int32, destinationAddress string, destinationPort int32) (*ConnectionOwner, error) {
	source, err := parseConnectionOwnerAddrPort(sourceAddress, sourcePort)
	if err != nil {
		return nil, E.Cause(err, "parse source")
	}
	destination, err := parseConnectionOwnerAddrPort(destinationAddress, destinationPort)
	if err != nil {
		return nil, E.Cause(err, "parse destination")
	}
	var network string
	switch ipProtocol {
	case syscall.IPPROTO_TCP:
		network = "tcp"
	case syscall.IPPROTO_UDP:
		network = "udp"
	default:
		return nil, E.New("unknown protocol: ", ipProtocol)
	}
	owner, err := process.FindDarwinConnectionOwner(network, source, destination)
	if err != nil {
		return nil, err
	}
	result := &ConnectionOwner{
		UserId:       owner.UserId,
		processPaths: owner.ProcessPaths,
	}
	if len(owner.ProcessPaths) > 0 {
		result.ProcessPath = owner.ProcessPaths[0]
	}
	if owner.UserId != -1 && owner.UserName == "" {
		osUser, _ := user.LookupId(F.ToString(owner.UserId))
		if osUser != nil {
			result.UserName = osUser.Username
		}
	}
	return result, nil
}

func parseConnectionOwnerAddrPort(address string, port int32) (netip.AddrPort, error) {
	if port < 0 || port > 65535 {
		return netip.AddrPort{}, E.New("invalid port: ", port)
	}
	addr, err := netip.ParseAddr(address)
	if err != nil {
		return netip.AddrPort{}, err
	}
	return netip.AddrPortFrom(addr.Unmap(), uint16(port)), nil
}
