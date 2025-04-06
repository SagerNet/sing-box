package transport

import (
	"context"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"

	mDNS "github.com/miekg/dns"
)

var _ adapter.DNSTransport = (*TLSTransport)(nil)

func RegisterTLS(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.RemoteTLSDNSServerOptions](registry, C.DNSTypeTLS, NewTLS)
}

type TLSTransport struct {
	dns.TransportAdapter
	logger      logger.ContextLogger
	dialer      N.Dialer
	serverAddr  M.Socksaddr
	tlsConfig   tls.Config
	access      sync.Mutex
	connections list.List[*tlsDNSConn]
}

type tlsDNSConn struct {
	tls.Conn
	queryId uint16
}

func NewTLS(ctx context.Context, logger log.ContextLogger, tag string, options option.RemoteTLSDNSServerOptions) (adapter.DNSTransport, error) {
	transportDialer, err := dns.NewRemoteDialer(ctx, options.RemoteDNSServerOptions)
	if err != nil {
		return nil, err
	}
	tlsOptions := common.PtrValueOrDefault(options.TLS)
	tlsOptions.Enabled = true
	tlsConfig, err := tls.NewClient(ctx, options.Server, tlsOptions)
	if err != nil {
		return nil, err
	}
	serverAddr := options.ServerOptions.Build()
	if serverAddr.Port == 0 {
		serverAddr.Port = 853
	}
	return &TLSTransport{
		TransportAdapter: dns.NewTransportAdapterWithRemoteOptions(C.DNSTypeTLS, tag, options.RemoteDNSServerOptions),
		logger:           logger,
		dialer:           transportDialer,
		serverAddr:       serverAddr,
		tlsConfig:        tlsConfig,
	}, nil
}

func (t *TLSTransport) Reset() {
	t.access.Lock()
	defer t.access.Unlock()
	for connection := t.connections.Front(); connection != nil; connection = connection.Next() {
		connection.Value.Close()
	}
	t.connections.Init()
}

func (t *TLSTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	t.access.Lock()
	conn := t.connections.PopFront()
	t.access.Unlock()
	if conn != nil {
		response, err := t.exchange(message, conn)
		if err == nil {
			return response, nil
		}
	}
	tcpConn, err := t.dialer.DialContext(ctx, N.NetworkTCP, t.serverAddr)
	if err != nil {
		return nil, err
	}
	tlsConn, err := tls.ClientHandshake(ctx, tcpConn, t.tlsConfig)
	if err != nil {
		tcpConn.Close()
		return nil, err
	}
	return t.exchange(message, &tlsDNSConn{Conn: tlsConn})
}

func (t *TLSTransport) exchange(message *mDNS.Msg, conn *tlsDNSConn) (*mDNS.Msg, error) {
	conn.queryId++
	err := WriteMessage(conn, conn.queryId, message)
	if err != nil {
		conn.Close()
		return nil, E.Cause(err, "write request")
	}
	response, err := ReadMessage(conn)
	if err != nil {
		conn.Close()
		return nil, E.Cause(err, "read response")
	}
	t.access.Lock()
	t.connections.PushBack(conn)
	t.access.Unlock()
	return response, nil
}
