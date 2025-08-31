package wsc

import (
	"context"
	"io"
	"net"
	"net/url"
	"sync"

	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"
)

var _ net.Conn = &clientConn{}

type clientConn struct {
	net.Conn
	reader *wsutil.Reader
	mu     sync.Mutex
}

func (cli *Client) newConn(ctx context.Context, network string, endpoint string) (*clientConn, error) {
	scheme := "ws"
	if cli.TLS != nil {
		scheme = "wss"
	}

	pURL := url.URL{
		Scheme:   scheme,
		Host:     cli.Host,
		Path:     cli.Path,
		RawQuery: "",
	}
	pQuery := pURL.Query()
	pQuery.Set("auth", cli.Auth)
	pQuery.Set("ep", endpoint)
	pQuery.Set("net", network)
	pURL.RawQuery = pQuery.Encode()

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

	reader := wsutil.NewReader(conn, ws.StateClientSide)

	return &clientConn{
		Conn:   conn,
		reader: reader,
	}, nil
}

func (conn *clientConn) Close() error {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	_ = wsutil.WriteClientMessage(conn.Conn, ws.OpClose, nil)
	return conn.Conn.Close()
}

func (conn *clientConn) Read(b []byte) (n int, err error) {
	err = nil
	var header ws.Header
	for {
		n, err = conn.reader.Read(b)
		if n > 0 {
			return
		}

		if !exceptions.IsMulti(err, io.EOF, wsutil.ErrNoFrameAdvance) {
			return
		}

		header, err = conn.reader.NextFrame()
		if err != nil {
			return
		}

		switch header.OpCode {
		case ws.OpBinary, ws.OpText, ws.OpContinuation:
			continue
		case ws.OpPing:
			wsutil.WriteClientMessage(conn.Conn, ws.OpPong, nil)
		case ws.OpPong:
			continue
		case ws.OpClose:
			err = io.EOF
			return
		default:
			continue
		}
	}
}

func (conn *clientConn) Write(b []byte) (n int, err error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if err := wsutil.WriteClientBinary(conn.Conn, b); err != nil {
		return 0, err
	}
	return len(b), nil
}
