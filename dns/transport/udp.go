package transport

import (
	"context"
	"net"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio/deadline"
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
	logger logger.ContextLogger

	dialer     N.Dialer
	serverAddr M.Socksaddr
	udpSize    atomic.Int32

	multiplexer *queryMultiplexer
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

func NewUDPRaw(logger logger.ContextLogger, adapter dns.TransportAdapter, dialerInstance N.Dialer, serverAddr M.Socksaddr) *UDPTransport {
	t := &UDPTransport{
		TransportAdapter: adapter,
		logger:           logger,
		dialer:           dialerInstance,
		serverAddr:       serverAddr,
	}
	t.udpSize.Store(2048)
	t.multiplexer = newQueryMultiplexer(queryMultiplexerOptions{
		dial: func(ctx context.Context) (net.Conn, error) {
			conn, err := t.dialer.DialContext(ctx, N.NetworkUDP, t.serverAddr)
			if err != nil {
				return nil, E.Cause(err, "dial UDP connection")
			}
			return conn, nil
		},
		write:    t.writeQuery,
		readNext: t.readResponse,
	})
	return t
}

func (t *UDPTransport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return dialer.InitializeDetour(t.dialer)
}

func (t *UDPTransport) Close() error {
	return t.multiplexer.Close()
}

func (t *UDPTransport) Reset() {
	t.multiplexer.Reset()
}

func (t *UDPTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	t.updateUDPSize(message)
	response, err := t.multiplexer.Exchange(ctx, message)
	if err != nil {
		return nil, err
	}
	if response.Truncated {
		t.logger.InfoContext(ctx, "response truncated, retrying with TCP")
		return t.exchangeTCP(ctx, message)
	}
	return response, nil
}

func (t *UDPTransport) ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	t.updateUDPSize(message)
	t.multiplexer.ExchangeAsync(ctx, message, func(response *mDNS.Msg, err error) {
		if err == nil && response.Truncated {
			t.logger.InfoContext(ctx, "response truncated, retrying with TCP")
			go func() {
				callback(t.exchangeTCP(ctx, message))
			}()
			return
		}
		callback(response, err)
	})
}

func (t *UDPTransport) updateUDPSize(message *mDNS.Msg) {
	edns0Opt := message.IsEdns0()
	if edns0Opt == nil {
		return
	}
	udpSize := int32(edns0Opt.UDPSize())
	for {
		current := t.udpSize.Load()
		if udpSize <= current {
			return
		}
		if t.udpSize.CompareAndSwap(current, udpSize) {
			t.Reset()
			return
		}
	}
}

func (t *UDPTransport) writeQuery(conn net.Conn, message *mDNS.Msg, queryId uint16) error {
	buffer := buf.NewSize(1 + message.Len())
	defer buffer.Release()
	exMessage := *message
	exMessage.Compress = true
	exMessage.Id = queryId
	rawMessage, err := exMessage.PackBuffer(buffer.FreeBytes())
	if err != nil {
		return err
	}
	return common.Error(conn.Write(rawMessage))
}

func (t *UDPTransport) readResponse(conn net.Conn) (*mDNS.Msg, error) {
	buffer := buf.NewSize(int(t.udpSize.Load()))
	defer buffer.Release()
	_, err := buffer.ReadOnceFrom(conn)
	if err != nil {
		return nil, err
	}
	var message mDNS.Msg
	err = message.Unpack(buffer.Bytes())
	if err != nil {
		t.logger.Debug("discarded malformed UDP response: ", err)
		return nil, nil
	}
	return &message, nil
}

func (t *UDPTransport) exchangeTCP(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	conn, err := t.dialer.DialContext(ctx, N.NetworkTCP, t.serverAddr)
	if err != nil {
		return nil, E.Cause(err, "dial TCP connection")
	}
	defer conn.Close()
	defer setConnDeadline(ctx, conn, deadline.NeedAdditionalReadDeadline(conn))()
	err = WriteMessage(conn, message.Id, message)
	if err != nil {
		return nil, E.Cause(err, "write request")
	}
	response, err := ReadMessage(conn)
	if err != nil {
		return nil, E.Cause(err, "read response")
	}
	return response, nil
}
