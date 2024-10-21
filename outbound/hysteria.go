//go:build with_quic

package outbound

import (
	"context"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/humanize"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-quic/hysteria"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ adapter.Outbound                = (*TUIC)(nil)
	_ adapter.InterfaceUpdateListener = (*TUIC)(nil)
)

type Hysteria struct {
	myOutboundAdapter
	client *hysteria.Client
}

func NewHysteria(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HysteriaOutboundOptions) (*Hysteria, error) {
	options.UDPFragmentDefault = true
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	tlsConfig, err := tls.NewClient(ctx, options.Server, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	outboundDialer, err := dialer.New(router, options.DialerOptions)
	if err != nil {
		return nil, err
	}
	networkList := options.Network.Build()
	var password string
	if options.AuthString != "" {
		password = options.AuthString
	} else {
		password = string(options.Auth)
	}
	var sendBps, receiveBps uint64
	if len(options.Up) > 0 {
		sendBps, err = humanize.ParseBytes(options.Up)
		if err != nil {
			return nil, E.Cause(err, "invalid up speed format: ", options.Up)
		}
	} else {
		sendBps = uint64(options.UpMbps) * hysteria.MbpsToBps
	}
	if len(options.Down) > 0 {
		receiveBps, err = humanize.ParseBytes(options.Down)
		if receiveBps == 0 {
			return nil, E.New("invalid down speed format: ", options.Down)
		}
	} else {
		receiveBps = uint64(options.DownMbps) * hysteria.MbpsToBps
	}
	client, err := hysteria.NewClient(hysteria.ClientOptions{
		Context:       ctx,
		Dialer:        outboundDialer,
		Logger:        logger,
		ServerAddress: options.ServerOptions.Build(),
		SendBPS:       sendBps,
		ReceiveBPS:    receiveBps,
		XPlusPassword: options.Obfs,
		Password:      password,
		TLSConfig:     tlsConfig,
		UDPDisabled:   !common.Contains(networkList, N.NetworkUDP),

		ConnReceiveWindow:   options.ReceiveWindowConn,
		StreamReceiveWindow: options.ReceiveWindow,
		DisableMTUDiscovery: options.DisableMTUDiscovery,
	})
	if err != nil {
		return nil, err
	}
	return &Hysteria{
		myOutboundAdapter: myOutboundAdapter{
			protocol:     C.TypeHysteria,
			network:      networkList,
			router:       router,
			logger:       logger,
			tag:          tag,
			dependencies: withDialerDependency(options.DialerOptions),
		},
		client: client,
	}, nil
}

func (h *Hysteria) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
		return h.client.DialConn(ctx, destination)
	case N.NetworkUDP:
		conn, err := h.ListenPacket(ctx, destination)
		if err != nil {
			return nil, err
		}
		return bufio.NewBindPacketConn(conn, destination), nil
	default:
		return nil, E.New("unsupported network: ", network)
	}
}

func (h *Hysteria) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	return h.client.ListenPacket(ctx, destination)
}

func (h *Hysteria) InterfaceUpdated() {
	h.client.CloseWithError(E.New("network changed"))
}

func (h *Hysteria) Close() error {
	return h.client.CloseWithError(os.ErrClosed)
}
