package outbound

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"sync"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"
)

var _ adapter.Outbound = &WSC{}

var _ net.Conn = &wscConn{}

type WSC struct {
	myOutboundAdapter
	dialer     N.Dialer
	serverAddr metadata.Socksaddr
	auth       string
	path       string
}

type wscConn struct {
	net.Conn
	reader *wsutil.Reader
	mu     sync.Mutex
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
		auth:       options.Auth,
		path:       options.Path,
	}
	if outbound.auth == "" {
		return nil, exceptions.New("Invalid Auth to use in authentications")
	}
	if !outbound.serverAddr.IsValid() {
		return nil, exceptions.New("Invalid server address")
	}
	if len(outbound.path) == 0 {
		outbound.path = "/"
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

	conn, err := wsc.newWscConn(ctx, wsc.auth, wsc.serverAddr, wsc.path, destination)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (wsc *WSC) ListenPacket(ctx context.Context, destination metadata.Socksaddr) (net.PacketConn, error) {
	ctx, meta := adapter.ExtendContext(ctx)
	meta.Outbound = wsc.tag
	meta.Destination = destination
	wsc.logger.InfoContext(ctx, "WSC outbound packet to ", destination)
	return wsc.dialer.ListenPacket(ctx, destination)
}

func (wsc *WSC) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	fmt.Println("new conn : ", metadata.Destination, " | ", metadata.Network, unsafe.Pointer(&conn))
	ctx = adapter.WithContext(ctx, &metadata)
	wsConn, err := wsc.DialContext(ctx, N.NetworkTCP, metadata.Destination)
	if err != nil {
		return N.ReportHandshakeFailure(conn, err)
	}

	if err = N.ReportHandshakeSuccess(conn); err != nil {
		wsConn.Close()
		return err
	}

	return CopyEarlyConn(ctx, conn, wsConn)
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
	return NewPacketConnection(ctx, wsc.dialer, conn, metadata)
}

func (wsc *WSC) Close() error {
	return nil
}

func (wsc *WSC) newWscConn(ctx context.Context, auth string, serverAddr metadata.Socksaddr, path string, endpoint metadata.Socksaddr) (*wscConn, error) {
	pURL := url.URL{
		Scheme:   "ws",
		Host:     serverAddr.String(),
		Path:     path,
		RawQuery: "",
	}
	pQuery := pURL.Query()
	pQuery.Set("auth", auth)
	pQuery.Set("ep", endpoint.String())
	pURL.RawQuery = pQuery.Encode()

	dialer := ws.Dialer{
		NetDial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return wsc.dialer.DialContext(ctx, N.NetworkTCP, metadata.ParseSocksaddr(addr))
		},
	}
	wsConn, _, _, err := dialer.Dial(ctx, pURL.String())
	if err != nil {
		return nil, err
	}
	// wsConn, _, _, err := ws.Dial(ctx, pURL.String())
	// if err != nil {
	// 	return nil, err
	// }

	reader := wsutil.NewReader(wsConn, ws.StateClientSide)

	return &wscConn{
		Conn:   wsConn,
		reader: reader,
	}, nil
}

func (cli *wscConn) Close() error {
	cli.mu.Lock()
	defer cli.mu.Unlock()
	_ = wsutil.WriteClientMessage(cli.Conn, ws.OpClose, nil)
	return cli.Conn.Close()
}

func (cli *wscConn) Read(b []byte) (n int, err error) {
	for {
		header, err := cli.reader.NextFrame()
		if err != nil {
			return 0, err
		}

		switch header.OpCode {
		case ws.OpBinary, ws.OpText, ws.OpContinuation:
			n, err := cli.reader.Read(b)
			if n > 0 {
				return n, nil
			}
			if err == io.EOF {
				continue
			}
			return n, err
		case ws.OpPing:
			wsutil.WriteClientMessage(cli.Conn, ws.OpPong, nil)
		case ws.OpPong:
			continue
		case ws.OpClose:
			return 0, io.EOF
		default:
			continue
		}
	}
}

func (cli *wscConn) Write(b []byte) (n int, err error) {
	cli.mu.Lock()
	defer cli.mu.Unlock()
	if err := wsutil.WriteClientBinary(cli.Conn, b); err != nil {
		return 0, err
	}
	return len(b), nil
}
