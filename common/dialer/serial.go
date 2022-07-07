package dialer

import (
	"context"
	"net"
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func DialSerial(ctx context.Context, dialer N.Dialer, network string, destination M.Socksaddr, destinationAddresses []netip.Addr) (net.Conn, error) {
	var conn net.Conn
	var err error
	var connErrors []error
	for _, address := range destinationAddresses {
		conn, err = dialer.DialContext(ctx, network, M.SocksaddrFromAddrPort(address, destination.Port))
		if err != nil {
			connErrors = append(connErrors, err)
		}
		return conn, nil
	}
	return nil, E.Errors(connErrors...)
}

func ListenSerial(ctx context.Context, dialer N.Dialer, destination M.Socksaddr, destinationAddresses []netip.Addr) (net.PacketConn, error) {
	var conn net.PacketConn
	var err error
	var connErrors []error
	for _, address := range destinationAddresses {
		conn, err = dialer.ListenPacket(ctx, M.SocksaddrFromAddrPort(address, destination.Port))
		if err != nil {
			connErrors = append(connErrors, err)
		}
		return conn, nil
	}
	return nil, E.Errors(connErrors...)
}
