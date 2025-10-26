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
	path       string
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
		path:    opts.Path,
		logger:  lg,
		dialer:  dialer,
		useTLS:  opts.TLS.Enabled,
	}

	if outbound.path == "" {
		outbound.path = "/"
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

func (out *Outbound) Type() string {
	return C.TypeWSC
}

func (out *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if network != N.NetworkTCP {
		return nil, errors.New("wsc: only TCP is supported")
	}

	host := out.serverAddr.Fqdn
	port := out.serverAddr.Port

	if host == "" && out.serverAddr.Fqdn == "" {
		host = out.serverAddr.Addr.String()
	}

	scheme := "ws"
	if out.useTLS {
		scheme = "wss"
	}

	uri := url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(host, fmt.Sprint(port)),
		Path:   "/wsc",
	}

	query := uri.Query()
	query.Set("user", out.account)
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

			return out.dialer.DialContext(ctx, N.NetworkTCP, M.Socksaddr{
				Fqdn: host,
				Port: uint16(portInt),
			})
		},
	}

	if out.useTLS && out.tlsCfg != nil {
		if (*out.tlsCfg).ServerName == "" {
			(*out.tlsCfg).ServerName = host
		}
		wsDialer.TLSConfig = out.tlsCfg
	}

	wsConn, _, _, err := wsDialer.Dial(ctx, uri.String())
	if err != nil {
		return nil, err
	}

	return newWSStreamConn(wsConn, false), nil
}

func (out *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("wsc: UDP is not supported")
}
