package wsc

import (
	"context"
	"net"
	"net/http"
	"net/url"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/ws"
)

var _ adapter.WSCClientTransport = &Client{}

type Client struct {
	Auth   string
	Host   string
	Path   string
	TLS    tls.Config
	Dialer N.Dialer
}

func (cli *Client) DialContext(ctx context.Context, network string, endpoint string) (net.Conn, error) {
	return cli.newConn(ctx, network, endpoint)
}

func (cli *Client) ListenPacket(ctx context.Context, network string, endpoint string) (net.PacketConn, error) {
	return cli.newPacketConn(ctx, network, endpoint)
}

func (cli *Client) Close(ctx context.Context) error {
	return cli.cleanup(ctx)
}

func (cli *Client) newWSConn(ctx context.Context, network string, endpoint string) (net.Conn, error) {
	pURL, _, err := cli.newURL("ws", "", endpoint, network)
	if err != nil {
		return nil, err
	}

	dialer := ws.Dialer{
		NetDial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := cli.Dialer.DialContext(ctx, N.NetworkTCP, metadata.ParseSocksaddr(addr))
			if err != nil {
				return nil, err
			}

			if cli.TLS != nil {
				conn, err = tls.ClientHandshake(ctx, conn, cli.TLS)
				if err != nil {
					return nil, err
				}
			}

			return conn, nil
		},
	}
	conn, _, _, err := dialer.Dial(ctx, pURL.String())
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (cli *Client) cleanup(ctx context.Context) error {
	pURL, tlsConfig, err := cli.newURL("http", "/cleanup", "", "")
	if err != nil {
		return err
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return cli.Dialer.DialContext(ctx, network, metadata.ParseSocksaddr(addr))
			},
		},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, pURL.String(), nil)
	if err != nil {
		return err
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return nil
}

func (cli *Client) newURL(scheme string, path string, endpoint string, network string) (url.URL, *tls.STDConfig, error) {
	var tlsConfig *tls.STDConfig = nil
	if cli.TLS != nil {
		scheme += "s"
		var err error
		tlsConfig, err = cli.TLS.Config()
		if err != nil {
			return url.URL{}, nil, err
		}
	}

	if path == "" {
		path = cli.Path
	}

	pURL := url.URL{
		Scheme:   scheme,
		Host:     cli.Host,
		Path:     path,
		RawQuery: "",
	}
	pQuery := pURL.Query()
	pQuery.Set("auth", cli.Auth)
	if endpoint != "" {
		pQuery.Set("ep", endpoint)
	}
	if network != "" {
		pQuery.Set("net", network)
	}
	pURL.RawQuery = pQuery.Encode()

	return pURL, tlsConfig, nil
}
