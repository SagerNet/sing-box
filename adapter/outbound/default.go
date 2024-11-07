package outbound

import (
	"context"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/canceler"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewConnection(ctx context.Context, this N.Dialer, conn net.Conn, metadata adapter.InboundContext) error {
	defer conn.Close()
	ctx = adapter.WithContext(ctx, &metadata)
	var outConn net.Conn
	var err error
	if len(metadata.DestinationAddresses) > 0 {
		outConn, err = N.DialSerial(ctx, this, N.NetworkTCP, metadata.Destination, metadata.DestinationAddresses)
	} else {
		outConn, err = this.DialContext(ctx, N.NetworkTCP, metadata.Destination)
	}
	if err != nil {
		return N.ReportHandshakeFailure(conn, err)
	}
	err = N.ReportConnHandshakeSuccess(conn, outConn)
	if err != nil {
		outConn.Close()
		return err
	}
	return CopyEarlyConn(ctx, conn, outConn)
}

func NewDirectConnection(ctx context.Context, router adapter.Router, this N.Dialer, conn net.Conn, metadata adapter.InboundContext, domainStrategy dns.DomainStrategy) error {
	defer conn.Close()
	ctx = adapter.WithContext(ctx, &metadata)
	var outConn net.Conn
	var err error
	if len(metadata.DestinationAddresses) > 0 {
		outConn, err = N.DialSerial(ctx, this, N.NetworkTCP, metadata.Destination, metadata.DestinationAddresses)
	} else if metadata.Destination.IsFqdn() {
		var destinationAddresses []netip.Addr
		destinationAddresses, err = router.Lookup(ctx, metadata.Destination.Fqdn, domainStrategy)
		if err != nil {
			return N.ReportHandshakeFailure(conn, err)
		}
		outConn, err = N.DialSerial(ctx, this, N.NetworkTCP, metadata.Destination, destinationAddresses)
	} else {
		outConn, err = this.DialContext(ctx, N.NetworkTCP, metadata.Destination)
	}
	if err != nil {
		return N.ReportHandshakeFailure(conn, err)
	}
	err = N.ReportConnHandshakeSuccess(conn, outConn)
	if err != nil {
		outConn.Close()
		return err
	}
	return CopyEarlyConn(ctx, conn, outConn)
}

func NewPacketConnection(ctx context.Context, this N.Dialer, conn N.PacketConn, metadata adapter.InboundContext) error {
	defer conn.Close()
	ctx = adapter.WithContext(ctx, &metadata)
	var (
		outPacketConn      net.PacketConn
		outConn            net.Conn
		destinationAddress netip.Addr
		err                error
	)
	if metadata.UDPConnect {
		if len(metadata.DestinationAddresses) > 0 {
			outConn, err = N.DialSerial(ctx, this, N.NetworkUDP, metadata.Destination, metadata.DestinationAddresses)
		} else {
			outConn, err = this.DialContext(ctx, N.NetworkUDP, metadata.Destination)
		}
		if err != nil {
			return N.ReportHandshakeFailure(conn, err)
		}
		outPacketConn = bufio.NewUnbindPacketConn(outConn)
		connRemoteAddr := M.AddrFromNet(outConn.RemoteAddr())
		if connRemoteAddr != metadata.Destination.Addr {
			destinationAddress = connRemoteAddr
		}
	} else {
		if len(metadata.DestinationAddresses) > 0 {
			outPacketConn, destinationAddress, err = N.ListenSerial(ctx, this, metadata.Destination, metadata.DestinationAddresses)
		} else {
			outPacketConn, err = this.ListenPacket(ctx, metadata.Destination)
		}
		if err != nil {
			return N.ReportHandshakeFailure(conn, err)
		}
	}
	err = N.ReportPacketConnHandshakeSuccess(conn, outPacketConn)
	if err != nil {
		outPacketConn.Close()
		return err
	}
	if destinationAddress.IsValid() {
		if metadata.Destination.IsFqdn() {
			if metadata.UDPDisableDomainUnmapping {
				outPacketConn = bufio.NewUnidirectionalNATPacketConn(bufio.NewPacketConn(outPacketConn), M.SocksaddrFrom(destinationAddress, metadata.Destination.Port), metadata.Destination)
			} else {
				outPacketConn = bufio.NewNATPacketConn(bufio.NewPacketConn(outPacketConn), M.SocksaddrFrom(destinationAddress, metadata.Destination.Port), metadata.Destination)
			}
		}
		if natConn, loaded := common.Cast[bufio.NATPacketConn](conn); loaded {
			natConn.UpdateDestination(destinationAddress)
		}
	}
	switch metadata.Protocol {
	case C.ProtocolSTUN:
		ctx, conn = canceler.NewPacketConn(ctx, conn, C.STUNTimeout)
	case C.ProtocolQUIC:
		ctx, conn = canceler.NewPacketConn(ctx, conn, C.QUICTimeout)
	case C.ProtocolDNS:
		ctx, conn = canceler.NewPacketConn(ctx, conn, C.DNSTimeout)
	}
	return bufio.CopyPacketConn(ctx, conn, bufio.NewPacketConn(outPacketConn))
}

func NewDirectPacketConnection(ctx context.Context, router adapter.Router, this N.Dialer, conn N.PacketConn, metadata adapter.InboundContext, domainStrategy dns.DomainStrategy) error {
	defer conn.Close()
	ctx = adapter.WithContext(ctx, &metadata)
	var (
		outPacketConn      net.PacketConn
		outConn            net.Conn
		destinationAddress netip.Addr
		err                error
	)
	if metadata.UDPConnect {
		if len(metadata.DestinationAddresses) > 0 {
			outConn, err = N.DialSerial(ctx, this, N.NetworkUDP, metadata.Destination, metadata.DestinationAddresses)
		} else if metadata.Destination.IsFqdn() {
			var destinationAddresses []netip.Addr
			destinationAddresses, err = router.Lookup(ctx, metadata.Destination.Fqdn, domainStrategy)
			if err != nil {
				return N.ReportHandshakeFailure(conn, err)
			}
			outConn, err = N.DialSerial(ctx, this, N.NetworkUDP, metadata.Destination, destinationAddresses)
		} else {
			outConn, err = this.DialContext(ctx, N.NetworkUDP, metadata.Destination)
		}
		if err != nil {
			return N.ReportHandshakeFailure(conn, err)
		}
		connRemoteAddr := M.AddrFromNet(outConn.RemoteAddr())
		if connRemoteAddr != metadata.Destination.Addr {
			destinationAddress = connRemoteAddr
		}
	} else {
		if len(metadata.DestinationAddresses) > 0 {
			outPacketConn, destinationAddress, err = N.ListenSerial(ctx, this, metadata.Destination, metadata.DestinationAddresses)
		} else if metadata.Destination.IsFqdn() {
			var destinationAddresses []netip.Addr
			destinationAddresses, err = router.Lookup(ctx, metadata.Destination.Fqdn, domainStrategy)
			if err != nil {
				return N.ReportHandshakeFailure(conn, err)
			}
			outPacketConn, destinationAddress, err = N.ListenSerial(ctx, this, metadata.Destination, destinationAddresses)
		} else {
			outPacketConn, err = this.ListenPacket(ctx, metadata.Destination)
		}
		if err != nil {
			return N.ReportHandshakeFailure(conn, err)
		}
	}
	err = N.ReportPacketConnHandshakeSuccess(conn, outPacketConn)
	if err != nil {
		outPacketConn.Close()
		return err
	}
	if destinationAddress.IsValid() {
		if metadata.Destination.IsFqdn() {
			outPacketConn = bufio.NewNATPacketConn(bufio.NewPacketConn(outPacketConn), M.SocksaddrFrom(destinationAddress, metadata.Destination.Port), metadata.Destination)
		}
		if natConn, loaded := common.Cast[bufio.NATPacketConn](conn); loaded {
			natConn.UpdateDestination(destinationAddress)
		}
	}
	switch metadata.Protocol {
	case C.ProtocolSTUN:
		ctx, conn = canceler.NewPacketConn(ctx, conn, C.STUNTimeout)
	case C.ProtocolQUIC:
		ctx, conn = canceler.NewPacketConn(ctx, conn, C.QUICTimeout)
	case C.ProtocolDNS:
		ctx, conn = canceler.NewPacketConn(ctx, conn, C.DNSTimeout)
	}
	return bufio.CopyPacketConn(ctx, conn, bufio.NewPacketConn(outPacketConn))
}

func CopyEarlyConn(ctx context.Context, conn net.Conn, serverConn net.Conn) error {
	if cachedReader, isCached := conn.(N.CachedReader); isCached {
		payload := cachedReader.ReadCached()
		if payload != nil && !payload.IsEmpty() {
			_, err := serverConn.Write(payload.Bytes())
			payload.Release()
			if err != nil {
				serverConn.Close()
				return err
			}
			return bufio.CopyConn(ctx, conn, serverConn)
		}
	}
	if earlyConn, isEarlyConn := common.Cast[N.EarlyConn](serverConn); isEarlyConn && earlyConn.NeedHandshake() {
		payload := buf.NewPacket()
		err := conn.SetReadDeadline(time.Now().Add(C.ReadPayloadTimeout))
		if err != os.ErrInvalid {
			if err != nil {
				payload.Release()
				serverConn.Close()
				return err
			}
			_, err = payload.ReadOnceFrom(conn)
			if err != nil && !E.IsTimeout(err) {
				payload.Release()
				serverConn.Close()
				return E.Cause(err, "read payload")
			}
			err = conn.SetReadDeadline(time.Time{})
			if err != nil {
				payload.Release()
				serverConn.Close()
				return err
			}
		}
		_, err = serverConn.Write(payload.Bytes())
		payload.Release()
		if err != nil {
			serverConn.Close()
			return N.ReportHandshakeFailure(conn, err)
		}
	}
	return bufio.CopyConn(ctx, conn, serverConn)
}
