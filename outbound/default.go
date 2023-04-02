package outbound

import (
	"context"
	"net"
	"net/netip"
	"os"
	"runtime"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/canceler"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

type myOutboundAdapter struct {
	protocol string
	network  []string
	router   adapter.Router
	logger   log.ContextLogger
	tag      string
}

func (a *myOutboundAdapter) Type() string {
	return a.protocol
}

func (a *myOutboundAdapter) Tag() string {
	return a.tag
}

func (a *myOutboundAdapter) Network() []string {
	return a.network
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
		return N.HandshakeFailure(conn, err)
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
		return N.HandshakeFailure(conn, err)
	}
	if destinationAddress.IsValid() {
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
			if err != nil {
				return err
			}
			return bufio.CopyConn(ctx, conn, serverConn)
		}
	}
	if earlyConn, isEarlyConn := common.Cast[N.EarlyConn](serverConn); isEarlyConn && earlyConn.NeedHandshake() {
		_payload := buf.StackNew()
		payload := common.Dup(_payload)
		err := conn.SetReadDeadline(time.Now().Add(C.ReadPayloadTimeout))
		if err != os.ErrInvalid {
			if err != nil {
				return err
			}
			_, err = payload.ReadOnceFrom(conn)
			if err != nil && !E.IsTimeout(err) {
				return E.Cause(err, "read payload")
			}
			err = conn.SetReadDeadline(time.Time{})
			if err != nil {
				payload.Release()
				return err
			}
		}
		_, err = serverConn.Write(payload.Bytes())
		if err != nil {
			return N.HandshakeFailure(conn, err)
		}
		runtime.KeepAlive(_payload)
		payload.Release()
	}
	return bufio.CopyConn(ctx, conn, serverConn)
}
