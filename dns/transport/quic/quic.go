package quic

import (
	"context"
	"errors"
	"sync"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing-box/adapter"
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
	"github.com/sagernet/sing/common/logger"
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
	ctx        context.Context
	logger     logger.ContextLogger
	dialer     N.Dialer
	serverAddr M.Socksaddr
	tlsConfig  tls.Config
	access     sync.Mutex
	connection *quic.Conn
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
		ctx:              ctx,
		logger:           logger,
		dialer:           transportDialer,
		serverAddr:       serverAddr,
		tlsConfig:        tlsConfig,
	}, nil
}

func (t *Transport) Start(stage adapter.StartStage) error {
	return nil
}

func (t *Transport) Close() error {
	t.access.Lock()
	defer t.access.Unlock()
	connection := t.connection
	if connection != nil {
		connection.CloseWithError(0, "")
	}
	return nil
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	var (
		conn     *quic.Conn
		err      error
		response *mDNS.Msg
	)
	for i := 0; i < 2; i++ {
		conn, err = t.openConnection()
		if err != nil {
			return nil, err
		}
		response, err = t.exchange(ctx, message, conn)
		if err == nil {
			return response, nil
		} else if !isQUICRetryError(err) {
			return nil, err
		} else {
			conn.CloseWithError(quic.ApplicationErrorCode(0), "")
			continue
		}
	}
	return nil, err
}

func (t *Transport) openConnection() (*quic.Conn, error) {
	connection := t.connection
	if connection != nil && !common.Done(connection.Context()) {
		return connection, nil
	}
	t.access.Lock()
	defer t.access.Unlock()
	connection = t.connection
	if connection != nil && !common.Done(connection.Context()) {
		return connection, nil
	}
	conn, err := t.dialer.DialContext(t.ctx, N.NetworkUDP, t.serverAddr)
	if err != nil {
		return nil, err
	}
	earlyConnection, err := sQUIC.DialEarly(
		t.ctx,
		bufio.NewUnbindPacketConn(conn),
		t.serverAddr.UDPAddr(),
		t.tlsConfig,
		nil,
	)
	if err != nil {
		return nil, err
	}
	t.connection = earlyConnection
	return earlyConnection, nil
}

func (t *Transport) exchange(ctx context.Context, message *mDNS.Msg, conn *quic.Conn) (*mDNS.Msg, error) {
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	err = transport.WriteMessage(stream, 0, message)
	if err != nil {
		stream.Close()
		return nil, err
	}
	stream.Close()
	return transport.ReadMessage(stream)
}

// https://github.com/AdguardTeam/dnsproxy/blob/fd1868577652c639cce3da00e12ca548f421baf1/upstream/upstream_quic.go#L394
func isQUICRetryError(err error) (ok bool) {
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
