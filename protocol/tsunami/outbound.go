package tsunami

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.TsunamiOutboundOptions](registry, C.TypeTsunami, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	dialer       tls.Dialer
	server       M.Socksaddr
	tlsConfig    tls.Config
	passwordHash [32]byte
	uotClient    *uot.Client
	logger       log.ContextLogger
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TsunamiOutboundOptions) (adapter.Outbound, error) {
	outbound := &Outbound{
		Adapter:      outbound.NewAdapterWithDialerOptions(C.TypeTsunami, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.DialerOptions),
		server:       options.ServerOptions.Build(),
		passwordHash: sha256.Sum256([]byte(options.Password)),
		logger:       logger,
	}
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	// Set default ALPN to h2 for TSUNAMI protocol
	if len(options.TLS.ALPN) == 0 {
		options.TLS.ALPN = []string{"h2"}
	}

	tlsConfig, err := tls.NewClient(ctx, logger, options.Server, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	outbound.tlsConfig = tlsConfig

	outboundDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:        ctx,
		Options:        options.DialerOptions,
		RemoteIsDomain: options.ServerIsDomain(),
	})
	if err != nil {
		return nil, err
	}

	outbound.dialer = tls.NewDialer(outboundDialer, tlsConfig)

	outbound.uotClient = &uot.Client{
		Dialer:  (tsunamiDialer)(outbound.createProxy),
		Version: uot.Version,
	}
	return outbound, nil
}

type tsunamiDialer func(ctx context.Context, destination M.Socksaddr) (net.Conn, error)

func (d tsunamiDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return d(ctx, destination)
}

func (d tsunamiDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

// createProxy establishes a TLS connection, authenticates, and writes the destination address.
func (h *Outbound) createProxy(ctx context.Context, destination M.Socksaddr) (net.Conn, error) {
	conn, err := h.dialer.DialTLSContext(ctx, h.server)
	if err != nil {
		return nil, err
	}

	// Send authentication: SHA-256(password) + padding_length(0)
	authBuf := make([]byte, 32+2)
	copy(authBuf[:32], h.passwordHash[:])
	binary.BigEndian.PutUint16(authBuf[32:34], 0) // no padding
	_, err = conn.Write(authBuf)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Write destination address in SOCKS5 format
	err = M.SocksaddrSerializer.WriteAddrPort(conn, destination)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func (h *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
		return h.createProxy(ctx, destination)
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
	return common.Close(h.tlsConfig)
}
