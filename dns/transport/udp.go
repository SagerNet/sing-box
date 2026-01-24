package transport

import (
	"context"
	"sync"
	"sync/atomic"

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
	*BaseTransport

	dialer     N.Dialer
	serverAddr M.Socksaddr
	udpSize    atomic.Int32

	connector *Connector[*Connection]

	callbackAccess sync.RWMutex
	queryId        uint16
	callbacks      map[uint16]*udpCallback
}

type udpCallback struct {
	access   sync.Mutex
	response *mDNS.Msg
	done     chan struct{}
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
		BaseTransport: NewBaseTransport(adapter, logger),
		dialer:        dialerInstance,
		serverAddr:    serverAddr,
		callbacks:     make(map[uint16]*udpCallback),
	}
	t.udpSize.Store(2048)
	t.connector = NewSingleflightConnector(t.CloseContext(), t.dial)
	return t
}

func (t *UDPTransport) dial(ctx context.Context) (*Connection, error) {
	rawConn, err := t.dialer.DialContext(ctx, N.NetworkUDP, t.serverAddr)
	if err != nil {
		return nil, E.Cause(err, "dial UDP connection")
	}
	conn := WrapConnection(rawConn)
	go t.recvLoop(conn)
	return conn, nil
}

func (t *UDPTransport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := t.SetStarted()
	if err != nil {
		return err
	}
	return dialer.InitializeDetour(t.dialer)
}

func (t *UDPTransport) Close() error {
	return E.Errors(t.BaseTransport.Close(), t.connector.Close())
}

func (t *UDPTransport) Reset() {
	t.connector.Reset()
}

func (t *UDPTransport) nextAvailableQueryId() (uint16, error) {
	start := t.queryId
	for {
		t.queryId++
		if _, exists := t.callbacks[t.queryId]; !exists {
			return t.queryId, nil
		}
		if t.queryId == start {
			return 0, E.New("no available query ID")
		}
	}
}

func (t *UDPTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	if !t.BeginQuery() {
		return nil, ErrTransportClosed
	}
	defer t.EndQuery()

	response, err := t.exchange(ctx, message)
	if err != nil {
		return nil, err
	}
	if response.Truncated {
		t.Logger.InfoContext(ctx, "response truncated, retrying with TCP")
		return t.exchangeTCP(ctx, message)
	}
	return response, nil
}

func (t *UDPTransport) exchangeTCP(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	conn, err := t.dialer.DialContext(ctx, N.NetworkTCP, t.serverAddr)
	if err != nil {
		return nil, E.Cause(err, "dial TCP connection")
	}
	defer conn.Close()
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

func (t *UDPTransport) exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	if edns0Opt := message.IsEdns0(); edns0Opt != nil {
		udpSize := int32(edns0Opt.UDPSize())
		for {
			current := t.udpSize.Load()
			if udpSize <= current {
				break
			}
			if t.udpSize.CompareAndSwap(current, udpSize) {
				t.connector.Reset()
				break
			}
		}
	}

	conn, err := t.connector.Get(ctx)
	if err != nil {
		return nil, err
	}

	callback := &udpCallback{
		done: make(chan struct{}),
	}

	t.callbackAccess.Lock()
	queryId, err := t.nextAvailableQueryId()
	if err != nil {
		t.callbackAccess.Unlock()
		return nil, err
	}
	t.callbacks[queryId] = callback
	t.callbackAccess.Unlock()

	defer func() {
		t.callbackAccess.Lock()
		delete(t.callbacks, queryId)
		t.callbackAccess.Unlock()
	}()

	buffer := buf.NewSize(1 + message.Len())
	defer buffer.Release()

	exMessage := *message
	exMessage.Compress = true
	originalId := message.Id
	exMessage.Id = queryId

	rawMessage, err := exMessage.PackBuffer(buffer.FreeBytes())
	if err != nil {
		return nil, err
	}

	_, err = conn.Write(rawMessage)
	if err != nil {
		conn.CloseWithError(err)
		return nil, E.Cause(err, "write request")
	}

	select {
	case <-callback.done:
		callback.response.Id = originalId
		return callback.response, nil
	case <-conn.Done():
		return nil, conn.CloseError()
	case <-t.CloseContext().Done():
		return nil, ErrTransportClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *UDPTransport) recvLoop(conn *Connection) {
	for {
		buffer := buf.NewSize(int(t.udpSize.Load()))
		_, err := buffer.ReadOnceFrom(conn)
		if err != nil {
			buffer.Release()
			conn.CloseWithError(err)
			return
		}

		var message mDNS.Msg
		err = message.Unpack(buffer.Bytes())
		buffer.Release()
		if err != nil {
			t.Logger.Debug("discarded malformed UDP response: ", err)
			continue
		}

		t.callbackAccess.RLock()
		callback, loaded := t.callbacks[message.Id]
		t.callbackAccess.RUnlock()

		if !loaded {
			continue
		}

		callback.access.Lock()
		select {
		case <-callback.done:
		default:
			callback.response = &message
			close(callback.done)
		}
		callback.access.Unlock()
	}
}
