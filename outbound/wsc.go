package outbound

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"time"

	"github.com/itsabgr/ge"
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

type WSC struct {
	myOutboundAdapter
	dialer N.Dialer
	auth   string
	host   string
	path   string
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
		auth:   options.Auth,
		host:   options.Host,
		path:   options.Path,
	}
	if len(outbound.auth) == 0 {
		return nil, exceptions.New("Invalid Auth to use in authentications")
	}
	if len(outbound.host) == 0 {
		return nil, exceptions.New("Invalid Host to connect websocket")
	}
	if len(outbound.path) == 0 {
		outbound.path = "/"
	}

	return outbound, nil
}

func (wsc *WSC) DialContext(ctx context.Context, network string, destination metadata.Socksaddr) (net.Conn, error) {
	wsc.logger.InfoContext(ctx, "WSC outbound connection to ", destination)
	return wsc.dialer.DialContext(ctx, N.NetworkName(network), destination)
}

func (wsc *WSC) ListenPacket(ctx context.Context, destination metadata.Socksaddr) (net.PacketConn, error) {
	wsc.logger.InfoContext(ctx, "WSC outbound packet to ", destination)
	return wsc.dialer.ListenPacket(ctx, destination)
}

func (wsc *WSC) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer conn.Close()

	pURL := url.URL{
		Scheme:   "ws",
		Host:     wsc.host,
		Path:     wsc.path,
		RawQuery: "",
	}
	pQuery := pURL.Query()
	pQuery.Set("auth", wsc.auth)
	pQuery.Set("ep", metadata.Destination.String())
	pURL.RawQuery = pQuery.Encode()

	wsConn, _, _, err := ws.Dial(ctx, pURL.String())
	if err != nil {
		return err
	}
	defer wsConn.Close()

	go func() {
		pack := make([]byte, 2048)
		for {
			if ctx.Err() != nil {
				return
			}

			if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				return
			}

			n, err := conn.Read(pack)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				if isTimeoutErr(err) {
					continue
				}
				return
			}

			if wErr := wsutil.WriteClientBinary(wsConn, pack[:n]); wErr != nil {
				return
			}
		}
	}()

	wsReader := wsutil.NewReader(wsConn, ws.StateClientSide)
	pack := make([]byte, 2048)
	for {
		if ctx.Err() != nil {
			return nil
		}

		if err := wsConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			return nil
		}

		header, err := wsReader.NextFrame()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if isTimeoutErr(err) {
				continue
			}
		}

		switch header.OpCode {
		case ws.OpPing:
			wsutil.WriteClientMessage(wsConn, ws.OpPong, nil)
			continue
		case ws.OpPong:
			continue
		case ws.OpClose:
			wsutil.WriteClientMessage(wsConn, ws.OpClose, nil)
			cancel()
			return exceptions.New("wsc websocket connection closed")
		}

		for {
			n, err := wsReader.Read(pack)
			if n > 0 {
				if _, wErr := conn.Write(pack[:n]); wErr != nil {
					return wErr
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return err
			}
		}
	}
}

func (wsc *WSC) NewPacketConnection(ctx context.Context, conn network.PacketConn, metadata adapter.InboundContext) error {
	fmt.Println("wsc packet conn: ", metadata, " | ", conn)
	return NewPacketConnection(ctx, wsc.dialer, conn, metadata)
}

func (wsc *WSC) Close() error {
	return nil
}

func isTimeoutErr(err error) bool {
	if nErr, ok := ge.As[net.Error](err); ok && nErr.Timeout() {
		return true
	}
	return false
}
