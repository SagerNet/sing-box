package outbound

import (
	"context"
	"fmt"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = &WSC{}

type WSC struct {
	myOutboundAdapter
	dialer     N.Dialer
	serverAddr metadata.Socksaddr
}

func NewWSC(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.WSCOutboundOptions) (*WSC, error) {
	outboundDialer, err := dialer.New(router, options.DialerOptions)
	if err != nil {
		return nil, err
	}

	outbound := &WSC{
		myOutboundAdapter: myOutboundAdapter{
			protocol:     C.TypeWSC,
			network:      options.Network.Build(),
			router:       router,
			logger:       logger,
			tag:          tag,
			dependencies: withDialerDependency(options.DialerOptions),
		},
		dialer:     outboundDialer,
		serverAddr: options.ServerOptions.Build(),
	}
	if !outbound.serverAddr.IsValid() {
		return nil, exceptions.New("Invalid server address")
	}

	return outbound, nil
}

func (wsc *WSC) DialContext(ctx context.Context, network string, destination metadata.Socksaddr) (net.Conn, error) {
	wsc.logger.InfoContext(ctx, "WSC outbound connection to ", destination)
	return wsc.dialer.DialContext(ctx, N.NetworkName(network), destination)
}

func (wsc *WSC) ListenPacket(ctx context.Context, destination metadata.Socksaddr) (net.PacketConn, error) {
	wsc.logger.InfoContext(ctx, "WSC outbound packet to ", destination)
	return wsc.dialer.ListenPacket(ctx, destination)
}

func (wsc *WSC) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	fmt.Println("wsc new conn: ", metadata, " | ", conn)
	data := make([]byte, 65000)
	n, err := conn.Read(data)
	if err != nil {
		return err
	}
	fmt.Println("data is : ", string(data[:n]))
	return NewConnection(ctx, wsc.dialer, conn, metadata)
}

func (wsc *WSC) NewPacketConnection(ctx context.Context, conn network.PacketConn, metadata adapter.InboundContext) error {
	fmt.Println("wsc packet conn: ", metadata, " | ", conn)
	return NewPacketConnection(ctx, wsc.dialer, conn, metadata)
}

func (wsc *WSC) Close() error {
	return nil
}
