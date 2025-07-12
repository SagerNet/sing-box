package transport

import (
	"context"
	"net"
	"os"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	mDNS "github.com/miekg/dns"
)

var _ adapter.DNSTransport = (*UDPTransport)(nil)

func RegisterUDP(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.RemoteDNSServerOptions](registry, C.DNSTypeUDP, NewUDP)
}

type UDPTransport struct {
	dns.TransportAdapter
	logger       logger.ContextLogger
	dialer       N.Dialer
	serverAddr   M.Socksaddr
	udpSize      int
	tcpTransport *TCPTransport
	access       sync.Mutex
	conn         *dnsConnection
	done         chan struct{}
}

func NewUDP(ctx context.Context, logger log.ContextLogger, tag string, options option.RemoteDNSServerOptions) (adapter.DNSTransport, error) {
	transportDialer, err := dns.NewRemoteDialer(ctx, options)
	if err != nil {
		return nil, err
	}
	serverAddr := options.DNSServerAddressOptions.Build()
	if serverAddr.Port == 0 {
		serverAddr.Port = 53
	}
	if !serverAddr.IsValid() {
		return nil, E.New("invalid server address: ", serverAddr)
	}
	return NewUDPRaw(logger, dns.NewTransportAdapterWithRemoteOptions(C.DNSTypeUDP, tag, options), transportDialer, serverAddr), nil
}

func NewUDPRaw(logger logger.ContextLogger, adapter dns.TransportAdapter, dialer N.Dialer, serverAddr M.Socksaddr) *UDPTransport {
	return &UDPTransport{
		TransportAdapter: adapter,
		logger:           logger,
		dialer:           dialer,
		serverAddr:       serverAddr,
		udpSize:          2048,
		tcpTransport: &TCPTransport{
			dialer:     dialer,
			serverAddr: serverAddr,
		},
		done: make(chan struct{}),
	}
}

func (t *UDPTransport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return dialer.InitializeDetour(t.dialer)
}

func (t *UDPTransport) Close() error {
	t.access.Lock()
	defer t.access.Unlock()
	close(t.done)
	t.done = make(chan struct{})
	return nil
}

func (t *UDPTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	response, err := t.exchange(ctx, message)
	if err != nil {
		return nil, err
	}
	if response.Truncated {
		t.logger.InfoContext(ctx, "response truncated, retrying with TCP")
		return t.tcpTransport.Exchange(ctx, message)
	}
	return response, nil
}

func (t *UDPTransport) exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	t.access.Lock()
	if edns0Opt := message.IsEdns0(); edns0Opt != nil {
		if udpSize := int(edns0Opt.UDPSize()); udpSize > t.udpSize {
			t.udpSize = udpSize
			close(t.done)
			t.done = make(chan struct{})
		}
	}
	t.access.Unlock()
	conn, err := t.open(ctx)
	if err != nil {
		return nil, err
	}
	buffer := buf.NewSize(1 + message.Len())
	defer buffer.Release()
	exMessage := *message
	exMessage.Compress = true
	messageId := message.Id
	callback := &dnsCallback{
		done: make(chan struct{}),
	}
	conn.access.Lock()
	conn.queryId++
	exMessage.Id = conn.queryId
	conn.callbacks[exMessage.Id] = callback
	conn.access.Unlock()
	defer func() {
		conn.access.Lock()
		delete(conn.callbacks, exMessage.Id)
		conn.access.Unlock()
	}()
	rawMessage, err := exMessage.PackBuffer(buffer.FreeBytes())
	if err != nil {
		return nil, err
	}
	_, err = conn.Write(rawMessage)
	if err != nil {
		conn.Close(err)
		return nil, err
	}
	select {
	case <-callback.done:
		callback.message.Id = messageId
		return callback.message, nil
	case <-conn.done:
		return nil, conn.err
	case <-t.done:
		return nil, os.ErrClosed
	case <-ctx.Done():
		conn.Close(ctx.Err())
		return nil, ctx.Err()
	}
}

func (t *UDPTransport) open(ctx context.Context) (*dnsConnection, error) {
	t.access.Lock()
	defer t.access.Unlock()
	if t.conn != nil {
		select {
		case <-t.conn.done:
		default:
			return t.conn, nil
		}
	}
	conn, err := t.dialer.DialContext(ctx, N.NetworkUDP, t.serverAddr)
	if err != nil {
		return nil, err
	}
	dnsConn := &dnsConnection{
		Conn:      conn,
		done:      make(chan struct{}),
		callbacks: make(map[uint16]*dnsCallback),
	}
	go t.recvLoop(dnsConn)
	t.conn = dnsConn
	return dnsConn, nil
}

func (t *UDPTransport) recvLoop(conn *dnsConnection) {
	for {
		buffer := buf.NewSize(t.udpSize)
		_, err := buffer.ReadOnceFrom(conn)
		if err != nil {
			buffer.Release()
			conn.Close(err)
			return
		}
		var message mDNS.Msg
		err = message.Unpack(buffer.Bytes())
		buffer.Release()
		if err != nil {
			conn.Close(err)
			return
		}
		conn.access.RLock()
		callback, loaded := conn.callbacks[message.Id]
		conn.access.RUnlock()
		if !loaded {
			continue
		}
		callback.access.Lock()
		select {
		case <-callback.done:
		default:
			callback.message = &message
			close(callback.done)
		}
		callback.access.Unlock()
	}
}

type dnsConnection struct {
	net.Conn
	access    sync.RWMutex
	done      chan struct{}
	closeOnce sync.Once
	err       error
	queryId   uint16
	callbacks map[uint16]*dnsCallback
}

func (c *dnsConnection) Close(err error) {
	c.closeOnce.Do(func() {
		c.err = err
		close(c.done)
	})
	c.Conn.Close()
}

type dnsCallback struct {
	access  sync.Mutex
	message *mDNS.Msg
	done    chan struct{}
}
