package dns

import (
	"context"
	"net/url"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewTransport(ctx context.Context, dialer N.Dialer, logger log.Logger, address string) (adapter.DNSTransport, error) {
	if address == "local" {
		return NewLocalTransport(), nil
	}
	serverURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	host := serverURL.Hostname()
	port := serverURL.Port()
	switch serverURL.Scheme {
	case "tls":
		if port == "" {
			port = "853"
		}
	default:
		if port == "" {
			port = "53"
		}
	}
	destination := M.ParseSocksaddrHostPortStr(host, port)
	switch serverURL.Scheme {
	case "", "udp":
		return NewUDPTransport(ctx, dialer, logger, destination), nil
	case "tcp":
		return NewTCPTransport(ctx, dialer, logger, destination), nil
	case "tls":
		return NewTLSTransport(ctx, dialer, logger, destination), nil
	case "https":
		return NewHTTPSTransport(dialer, serverURL.String()), nil
	default:
		return nil, E.New("unknown dns scheme: " + serverURL.Scheme)
	}
}
