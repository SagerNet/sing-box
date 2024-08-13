package outbound

import (
	"context"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/canceler"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type myOutboundAdapter struct {
	protocol     string
	network      []string
	router       adapter.Router
	logger       log.ContextLogger
	tag          string
	port         uint16
	dependencies []string
}

func (a *myOutboundAdapter) Type() string {
	return a.protocol
}

func (a *myOutboundAdapter) Tag() string {
	return a.tag
}

func (a *myOutboundAdapter) Port() int {
	switch a.protocol {
	case C.TypeDirect, C.TypeBlock, C.TypeDNS, C.TypeTor, C.TypeSelector, C.TypeURLTest:
		return 65536
	default:
		return int(a.port)
	}
}

func (a *myOutboundAdapter) SetTag(tag string) {
	a.tag = tag
}

func (a *myOutboundAdapter) Network() []string {
	return a.network
}

func (a *myOutboundAdapter) Dependencies() []string {
	return a.dependencies
}

func (a *myOutboundAdapter) NewError(ctx context.Context, err error) {
	NewError(a.logger, ctx, err)
}

func withDialerDependency(options option.DialerOptions) []string {
	if options.Detour != "" {
		return []string{options.Detour}
	}
	return nil
}

func NewConnection(ctx context.Context, this N.Dialer, conn net.Conn, metadata adapter.InboundContext) error {
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
	err = N.ReportHandshakeSuccess(conn)
	if err != nil {
		outConn.Close()
		return err
	}
	return CopyEarlyConn(ctx, conn, outConn)
}

func NewDirectConnection(ctx context.Context, router adapter.Router, this N.Dialer, conn net.Conn, metadata adapter.InboundContext, domainStrategy dns.DomainStrategy) error {
	ctx = adapter.WithContext(ctx, &metadata)
	var outConn net.Conn
	var err error
	addresses := metadata.DestinationAddresses
	if len(addresses) == 0 && metadata.Destination.IsFqdn() {
		addresses, err = router.Lookup(ctx, metadata.Destination.Fqdn, domainStrategy)
		if err != nil {
			return N.ReportHandshakeFailure(conn, err)
		}
	}
	if len(addresses) > 0 {
		addresses4 := common.Filter(addresses, func(address netip.Addr) bool {
			return address.Is4() || address.Is4In6()
		})
		addresses6 := common.Filter(addresses, func(address netip.Addr) bool {
			return address.Is6() && !address.Is4In6()
		})
		connFunc := func(primaries []netip.Addr, fallbacks []netip.Addr) (net.Conn, error) {
			if len(primaries) > 0 {
				if conn, err := N.DialSerial(ctx, this, N.NetworkTCP, metadata.Destination, primaries); err == nil || len(fallbacks) == 0 {
					return conn, err
				}
			}
			return N.DialSerial(ctx, this, N.NetworkTCP, metadata.Destination, fallbacks)
		}
		switch domainStrategy {
		case dns.DomainStrategyAsIS:
			outConn, err = connFunc(addresses, nil)
		case dns.DomainStrategyUseIPv4:
			outConn, err = connFunc(addresses4, nil)
		case dns.DomainStrategyUseIPv6:
			outConn, err = connFunc(addresses6, nil)
		case dns.DomainStrategyPreferIPv4:
			outConn, err = connFunc(addresses4, addresses6)
		case dns.DomainStrategyPreferIPv6:
			outConn, err = connFunc(addresses6, addresses4)
		}
	} else {
		outConn, err = this.DialContext(ctx, N.NetworkTCP, metadata.Destination)
	}
	if err != nil {
		return N.ReportHandshakeFailure(conn, err)
	}
	err = N.ReportHandshakeSuccess(conn)
	if err != nil {
		outConn.Close()
		return err
	}
	return CopyEarlyConn(ctx, conn, outConn)
}

func NewPacketConnection(ctx context.Context, this N.Dialer, conn N.PacketConn, metadata adapter.InboundContext) error {
	ctx = adapter.WithContext(ctx, &metadata)
	var outConn net.PacketConn
	var destinationAddress netip.Addr
	var err error
	if len(metadata.DestinationAddresses) > 0 {
		outConn, destinationAddress, err = N.ListenSerial(ctx, this, metadata.Destination, metadata.DestinationAddresses)
	} else {
		outConn, err = this.ListenPacket(ctx, metadata.Destination)
	}
	if err != nil {
		return N.ReportHandshakeFailure(conn, err)
	}
	err = N.ReportHandshakeSuccess(conn)
	if err != nil {
		outConn.Close()
		return err
	}
	if destinationAddress.IsValid() {
		if metadata.Destination.IsFqdn() {
			if metadata.InboundOptions.UDPDisableDomainUnmapping {
				outConn = bufio.NewUnidirectionalNATPacketConn(bufio.NewPacketConn(outConn), M.SocksaddrFrom(destinationAddress, metadata.Destination.Port), metadata.Destination)
			} else {
				outConn = bufio.NewNATPacketConn(bufio.NewPacketConn(outConn), M.SocksaddrFrom(destinationAddress, metadata.Destination.Port), metadata.Destination)
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
	return bufio.CopyPacketConn(ctx, conn, bufio.NewPacketConn(outConn))
}

func NewDirectPacketConnection(ctx context.Context, router adapter.Router, this N.Dialer, conn N.PacketConn, metadata adapter.InboundContext, domainStrategy dns.DomainStrategy) error {
	ctx = adapter.WithContext(ctx, &metadata)
	var outConn net.PacketConn
	var destinationAddress netip.Addr
	var err error
	addresses := metadata.DestinationAddresses
	if len(addresses) == 0 && metadata.Destination.IsFqdn() {
		addresses, err = router.Lookup(ctx, metadata.Destination.Fqdn, domainStrategy)
		if err != nil {
			return N.ReportHandshakeFailure(conn, err)
		}
	}
	if len(addresses) > 0 {
		addresses4 := common.Filter(addresses, func(address netip.Addr) bool {
			return address.Is4() || address.Is4In6()
		})
		addresses6 := common.Filter(addresses, func(address netip.Addr) bool {
			return address.Is6() && !address.Is4In6()
		})
		connFunc := func(primaries []netip.Addr, fallbacks []netip.Addr) (net.PacketConn, netip.Addr, error) {
			if len(primaries) > 0 {
				if conn, addr, err := N.ListenSerial(ctx, this, metadata.Destination, primaries); err == nil || len(fallbacks) == 0 {
					return conn, addr, err
				}
			}
			return N.ListenSerial(ctx, this, metadata.Destination, fallbacks)
		}
		switch domainStrategy {
		case dns.DomainStrategyAsIS:
			outConn, destinationAddress, err = connFunc(addresses, nil)
		case dns.DomainStrategyUseIPv4:
			outConn, destinationAddress, err = connFunc(addresses4, nil)
		case dns.DomainStrategyUseIPv6:
			outConn, destinationAddress, err = connFunc(addresses6, nil)
		case dns.DomainStrategyPreferIPv4:
			outConn, destinationAddress, err = connFunc(addresses4, addresses6)
		case dns.DomainStrategyPreferIPv6:
			outConn, destinationAddress, err = connFunc(addresses6, addresses4)
		}
	} else {
		outConn, err = this.ListenPacket(ctx, metadata.Destination)
	}
	if err != nil {
		return N.ReportHandshakeFailure(conn, err)
	}
	err = N.ReportHandshakeSuccess(conn)
	if err != nil {
		outConn.Close()
		return err
	}
	if destinationAddress.IsValid() {
		if metadata.Destination.IsFqdn() {
			outConn = bufio.NewNATPacketConn(bufio.NewPacketConn(outConn), M.SocksaddrFrom(destinationAddress, metadata.Destination.Port), metadata.Destination)
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
	return bufio.CopyPacketConn(ctx, conn, bufio.NewPacketConn(outConn))
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

func NewError(logger log.ContextLogger, ctx context.Context, err error) {
	common.Close(err)
	if E.IsClosedOrCanceled(err) {
		logger.DebugContext(ctx, "connection closed: ", err)
		return
	}
	logger.ErrorContext(ctx, err)
}
