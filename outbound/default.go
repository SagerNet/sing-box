package outbound

import (
	"context"
	"net"
	"runtime"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

type myOutboundAdapter struct {
	protocol string
	logger   log.ContextLogger
	tag      string
	network  []string
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
		outConn, err = N.DialSerial(ctx, this, C.NetworkTCP, metadata.Destination, metadata.DestinationAddresses)
	} else {
		outConn, err = this.DialContext(ctx, C.NetworkTCP, metadata.Destination)
	}
	if err != nil {
		return err
	}
	return bufio.CopyConn(ctx, conn, outConn)
}

func NewEarlyConnection(ctx context.Context, this N.Dialer, conn net.Conn, metadata adapter.InboundContext) error {
	ctx = adapter.WithContext(ctx, &metadata)
	var outConn net.Conn
	var err error
	if len(metadata.DestinationAddresses) > 0 {
		outConn, err = N.DialSerial(ctx, this, C.NetworkTCP, metadata.Destination, metadata.DestinationAddresses)
	} else {
		outConn, err = this.DialContext(ctx, C.NetworkTCP, metadata.Destination)
	}
	if err != nil {
		return err
	}
	return CopyEarlyConn(ctx, conn, outConn)
}

func NewPacketConnection(ctx context.Context, this N.Dialer, conn N.PacketConn, metadata adapter.InboundContext) error {
	ctx = adapter.WithContext(ctx, &metadata)
	var outConn net.PacketConn
	var err error
	if len(metadata.DestinationAddresses) > 0 {
		outConn, err = N.ListenSerial(ctx, this, metadata.Destination, metadata.DestinationAddresses)
	} else {
		outConn, err = this.ListenPacket(ctx, metadata.Destination)
	}
	if err != nil {
		return err
	}
	return bufio.CopyPacketConn(ctx, conn, bufio.NewPacketConn(outConn))
}

func CopyEarlyConn(ctx context.Context, conn net.Conn, serverConn net.Conn) error {
	if cachedReader, isCached := serverConn.(N.CachedReader); isCached {
		payload := cachedReader.ReadCached()
		if payload != nil && !payload.IsEmpty() {
			_, err := serverConn.Write(payload.Bytes())
			if err != nil {
				return err
			}
			return bufio.CopyConn(ctx, conn, serverConn)
		}
	}
	_payload := buf.StackNew()
	payload := common.Dup(_payload)
	err := conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if err != nil {
		return err
	}
	_, err = payload.ReadFrom(conn)
	if err != nil && !E.IsTimeout(err) {
		return E.Cause(err, "read payload")
	}
	err = conn.SetReadDeadline(time.Time{})
	if err != nil {
		payload.Release()
		return err
	}
	_, err = serverConn.Write(payload.Bytes())
	if err != nil {
		return E.Cause(err, "client handshake")
	}
	runtime.KeepAlive(_payload)
	return bufio.CopyConn(ctx, conn, serverConn)
}
