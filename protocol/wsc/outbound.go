package wsc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/sagernet/ws"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register(registry, C.TypeWSC, NewOutbound)
}

type Outbound struct {
	outbound.Adapter

	account    string
	logger     logger.ContextLogger
	serverAddr M.Socksaddr
	dialer     N.Dialer
	tlsCfg     *tls.Config
	useTLS     bool
}

func NewOutbound(ctx context.Context, router adapter.Router, lg log.ContextLogger, tag string, opts option.WSCOutboundOptions) (adapter.Outbound, error) {
	dialer, err := dialer.New(ctx, opts.DialerOptions, opts.ServerIsDomain())
	if err != nil {
		return nil, err
	}

	outbound := &Outbound{
		Adapter: outbound.NewAdapterWithDialerOptions(
			C.TypeWSC, tag, []string{N.NetworkTCP}, opts.DialerOptions,
		),
		account: opts.Auth,
		logger:  lg,
		dialer:  dialer,
		useTLS:  opts.TLS.Enabled,
	}

	if opts.TLS.Enabled {
		outbound.tlsCfg = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	outbound.serverAddr = opts.ServerOptions.Build()
	if outbound.serverAddr.Port == 0 {
		return nil, errors.New("port is not specified")
	}
	return outbound, nil
}

func (outbound *Outbound) Type() string {
	return C.TypeWSC
}

func (outbound *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if network != N.NetworkTCP {
		return nil, errors.New("wsc: only TCP is supported")
	}

	host := outbound.serverAddr.Fqdn
	port := outbound.serverAddr.Port

	if host == "" && outbound.serverAddr.Fqdn == "" {
		host = outbound.serverAddr.Addr.String()
	}

	scheme := "ws"
	if outbound.useTLS {
		scheme = "wss"
	}

	uri := url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(host, fmt.Sprint(port)),
		Path:   "/wsc",
	}

	query := uri.Query()
	query.Set("user", outbound.account)
	query.Set("net", "tcp")
	query.Set("addr", destination.String())
	uri.RawQuery = query.Encode()

	wsDialer := ws.Dialer{
		NetDial: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}

			portInt, err := strconv.ParseUint(port, 10, 16)
			if err != nil {
				return nil, err
			}

			return outbound.dialer.DialContext(ctx, N.NetworkTCP, M.Socksaddr{
				Fqdn: host,
				Port: uint16(portInt),
			})
		},
	}

	if outbound.useTLS && outbound.tlsCfg != nil {
		if (*outbound.tlsCfg).ServerName == "" {
			(*outbound.tlsCfg).ServerName = host
		}
		wsDialer.TLSConfig = outbound.tlsCfg
	}

	wsConn, _, _, err := wsDialer.Dial(ctx, uri.String())
	if err != nil {
		return nil, err
	}

	return newWSStreamConn(wsConn), nil
}

func (outbound *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("wsc: UDP is not supported")
}
