package quic

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	sQUIC "github.com/sagernet/sing-quic"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	mDNS "github.com/miekg/dns"
)

var _ adapter.DNSTransport = (*Transport)(nil)

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.RemoteTLSDNSServerOptions](registry, C.DNSTypeQUIC, NewQUIC)
}

type Transport struct {
	dns.TransportAdapter

	dialer     N.Dialer
	serverAddr M.Socksaddr
	tlsConfig  tls.Config

	connection *transport.ConnPool[*quic.Conn]
}

func NewQUIC(ctx context.Context, logger log.ContextLogger, tag string, options option.RemoteTLSDNSServerOptions) (adapter.DNSTransport, error) {
	transportDialer, err := dns.NewRemoteDialer(ctx, options.RemoteDNSServerOptions)
	if err != nil {
		return nil, err
	}
	tlsOptions := common.PtrValueOrDefault(options.TLS)
	tlsOptions.Enabled = true
	tlsConfig, err := tls.NewClient(ctx, logger, options.Server, tlsOptions)
	if err != nil {
		return nil, err
	}
	if len(tlsConfig.NextProtos()) == 0 {
		tlsConfig.SetNextProtos([]string{"doq"})
	}
	serverAddr := options.DNSServerAddressOptions.Build()
	if serverAddr.Port == 0 {
		serverAddr.Port = 853
	}
	if !serverAddr.IsValid() {
		return nil, E.New("invalid server address: ", serverAddr)
	}

	return &Transport{
		TransportAdapter: dns.NewTransportAdapterWithRemoteOptions(C.DNSTypeQUIC, tag, options.RemoteDNSServerOptions),
		dialer:           transportDialer,
		serverAddr:       serverAddr,
		tlsConfig:        tlsConfig,
		connection: transport.NewConnPool(transport.ConnPoolOptions[*quic.Conn]{
			Mode: transport.ConnPoolSingle,
			IsAlive: func(conn *quic.Conn) bool {
				return conn != nil && !common.Done(conn.Context())
			},
			Close: func(conn *quic.Conn, _ error) {
				conn.CloseWithError(0, "")
			},
		}),
	}, nil
}

func (t *Transport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return dialer.InitializeDetour(t.dialer)
}

func (t *Transport) Close() error {
	return t.connection.Close()
}

func (t *Transport) Reset() {
	t.connection.Reset()
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	var (
		conn     *quic.Conn
		err      error
		response *mDNS.Msg
	)
	for range 2 {
		conn, _, err = t.connection.Acquire(ctx, func(ctx context.Context) (*quic.Conn, error) {
			rawConn, err := t.dialer.DialContext(ctx, N.NetworkUDP, t.serverAddr)
			if err != nil {
				return nil, E.Cause(err, "dial UDP connection")
			}
			earlyConnection, err := sQUIC.DialEarly(
				ctx,
				bufio.NewUnbindPacketConn(rawConn),
				t.serverAddr.UDPAddr(),
				t.tlsConfig,
				nil,
			)
			if err != nil {
				rawConn.Close()
				return nil, E.Cause(err, "establish QUIC connection")
			}
			// quic-go does not take ownership of the packet conn passed to
			// DialEarly: when the connection ends it only stops reading.
			go func() {
				<-earlyConnection.Context().Done()
				rawConn.Close()
			}()
			return earlyConnection, nil
		})
		if err != nil {
			return nil, err
		}
		response, err = t.exchange(ctx, message, conn)
		if err == nil {
			t.connection.Release(conn, true)
			return response, nil
		} else if !isQUICRetryError(err) {
			t.connection.Release(conn, true)
			return nil, err
		} else {
			t.connection.Release(conn, true)
			t.Reset()
			continue
		}
	}
	return nil, err
}

func (t *Transport) ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	go func() {
		callback(t.Exchange(ctx, message))
	}()
}

func (t *Transport) exchange(ctx context.Context, message *mDNS.Msg, conn *quic.Conn) (*mDNS.Msg, error) {
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, E.Cause(err, "open stream")
	}
	defer stream.CancelRead(0)
	stopWatch := context.AfterFunc(ctx, func() {
		stream.CancelRead(0)
		_ = stream.SetWriteDeadline(time.Now())
	})
	defer stopWatch()
	err = transport.WriteMessage(stream, 0, message)
	if err != nil {
		stream.Close()
		return nil, E.Cause(err, "write request")
	}
	stream.Close()
	response, err := transport.ReadMessage(stream)
	if err != nil {
		return nil, E.Cause(err, "read response")
	}
	return response, nil
}

// https://github.com/AdguardTeam/dnsproxy/blob/fd1868577652c639cce3da00e12ca548f421baf1/upstream/upstream_quic.go#L394
func isQUICRetryError(err error) (ok bool) {
	if errors.Is(err, os.ErrClosed) {
		return true
	}

	var qAppErr *quic.ApplicationError
	if errors.As(err, &qAppErr) && qAppErr.ErrorCode == 0 {
		return true
	}

	var qIdleErr *quic.IdleTimeoutError
	if errors.As(err, &qIdleErr) {
		return true
	}

	var resetErr *quic.StatelessResetError
	if errors.As(err, &resetErr) {
		return true
	}

	var qTransportError *quic.TransportError
	if errors.As(err, &qTransportError) && qTransportError.ErrorCode == quic.NoError {
		return true
	}

	if errors.Is(err, quic.Err0RTTRejected) {
		return true
	}

	return false
}
