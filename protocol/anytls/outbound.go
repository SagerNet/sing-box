package anytls

import (
	"context"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/anytls"
	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.AnyTLSOutboundOptions](registry, C.TypeAnyTLS, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	client    *anytls.Client
	uotClient *uot.Client
	logger    log.ContextLogger
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.AnyTLSOutboundOptions) (adapter.Outbound, error) {
	outbound := &Outbound{
		Adapter: outbound.NewAdapterWithDialerOptions(C.TypeAnyTLS, tag, []string{N.NetworkTCP}, options.DialerOptions),
		logger:  logger,
	}
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}

	tlsConfig, err := tls.NewClient(ctx, options.Server, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}

	outboundDialer, err := dialer.NewWithOptions(dialer.Options{
		Context: ctx,
		Options: options.DialerOptions,
	})
	if err != nil {
		return nil, err
	}
	client, err := anytls.NewClient(ctx, anytls.ClientConfig{
		Password:                 options.Password,
		IdleSessionCheckInterval: options.IdleSessionCheckInterval.Build(),
		IdleSessionTimeout:       options.IdleSessionTimeout.Build(),
		Server:                   options.ServerOptions.Build(),
		Dialer:                   outboundDialer,
		TLSConfig:                tlsConfig,
		Logger:                   logger,
	})
	if err != nil {
		return nil, err
	}
	outbound.client = client

	outbound.uotClient = &uot.Client{
		Dialer:  outbound,
		Version: uot.Version,
	}
	return outbound, nil
}

func (h *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
		return h.client.CreateProxy(ctx, destination)
	case N.NetworkUDP:
		h.logger.InfoContext(ctx, "outbound UoT packet connection to ", destination)
		return h.uotClient.DialContext(ctx, network, destination)
	}
	return nil, os.ErrInvalid
}

func (h *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	h.logger.InfoContext(ctx, "outbound UoT packet connection to ", destination)
	return h.uotClient.ListenPacket(ctx, destination)
}

func (h *Outbound) Close() error {
	return common.Close(h.client)
}
