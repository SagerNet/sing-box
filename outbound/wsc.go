package outbound

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/wsc"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = &WSC{}

type WSC struct {
	myOutboundAdapter
	dialer    N.Dialer
	tlsConfig tls.Config
	client    adapter.WSCClientTransport
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
		dialer: outboundDialer,
	}

	serverAddr := options.ServerOptions.Build()

	if options.Auth == "" {
		return nil, exceptions.New("Invalid Auth to use in authentications")
	}
	if !serverAddr.IsValid() {
		return nil, exceptions.New("Invalid server address")
	}
	if options.Path == "" {
		options.Path = "/"
	}
	if options.TLS != nil {
		outbound.tlsConfig, err = tls.NewClient(ctx, options.Server, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
	}

	outbound.client = &wsc.Client{
		Auth:   options.Auth,
		Host:   serverAddr.String(),
		Path:   options.Path,
		TLS:    outbound.tlsConfig,
		Dialer: outbound.dialer,
	}

	return outbound, nil
}

func (wsc *WSC) DialContext(ctx context.Context, network string, destination metadata.Socksaddr) (net.Conn, error) {
	ctx, meta := adapter.ExtendContext(ctx)
	meta.Outbound = wsc.tag
	meta.Destination = destination
	if N.NetworkName(network) != N.NetworkTCP {
		return nil, exceptions.Extend(N.ErrUnknownNetwork, network)
	}
	wsc.logger.InfoContext(ctx, "WSC outbound connection to ", destination)
	return wsc.client.DialContext(ctx, network, destination.String())
}

func (wsc *WSC) ListenPacket(ctx context.Context, destination metadata.Socksaddr) (net.PacketConn, error) {
	ctx, meta := adapter.ExtendContext(ctx)
	meta.Outbound = wsc.tag
	meta.Destination = destination
	wsc.logger.InfoContext(ctx, "WSC outbound packet to ", destination)
	return wsc.dialer.ListenPacket(ctx, destination)
}

func (wsc *WSC) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, wsc, conn, metadata)
}

func (wsc *WSC) NewPacketConnection(ctx context.Context, conn network.PacketConn, metadata adapter.InboundContext) error {
	fmt.Println("new packet conn: ", metadata)
	// fmt.Println("wsc packet conn: ", metadata, " | ", conn)
	// buffer := buf.NewPacket()
	// defer buffer.Release()
	// dest, err := conn.ReadPacket(buffer)
	// if err != nil {
	// 	fmt.Println("error wsc packet conn: ", err)
	// 	return err
	// }
	// fmt.Println("wsc packet conn data is : ", dest, " | ", dest.Network(), " | ", buffer.Len())

	// time.Sleep(time.Second * 10)
	// return NewPacketConnection(ctx, wsc.dialer, conn, metadata)
	return NewPacketConnection(ctx, wsc, conn, metadata)
}

func (wsc *WSC) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return wsc.client.Close(ctx)
}
