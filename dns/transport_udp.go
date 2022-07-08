package dns

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/task"

	"golang.org/x/net/dns/dnsmessage"
)

var _ adapter.DNSTransport = (*UDPTransport)(nil)

type UDPTransport struct {
	myTransportAdapter
}

func NewUDPTransport(ctx context.Context, dialer N.Dialer, logger log.Logger, destination M.Socksaddr) *UDPTransport {
	return &UDPTransport{
		myTransportAdapter{
			ctx:         ctx,
			dialer:      dialer,
			logger:      logger,
			destination: destination,
			done:        make(chan struct{}),
		},
	}
}

func (t *UDPTransport) offer() (*dnsConnection, error) {
	t.access.RLock()
	connection := t.connection
	t.access.RUnlock()
	if connection != nil {
		select {
		case <-connection.done:
		default:
			return connection, nil
		}
	}
	t.access.Lock()
	connection = t.connection
	if connection != nil {
		select {
		case <-connection.done:
		default:
			t.access.Unlock()
			return connection, nil
		}
	}
	tcpConn, err := t.dialer.DialContext(t.ctx, "udp", t.destination)
	if err != nil {
		return nil, err
	}
	connection = &dnsConnection{
		Conn:      tcpConn,
		done:      make(chan struct{}),
		callbacks: make(map[uint16]chan *dnsmessage.Message),
	}
	t.connection = connection
	t.access.Unlock()
	go t.newConnection(connection)
	return connection, nil
}

func (t *UDPTransport) newConnection(conn *dnsConnection) {
	defer close(conn.done)
	defer conn.Close()
	err := task.Any(t.ctx, func(ctx context.Context) error {
		return t.loopIn(conn)
	}, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return nil
		case <-t.done:
			return os.ErrClosed
		}
	})
	conn.err = err
	if err != nil {
		t.logger.Debug("connection closed: ", err)
	}
}

func (t *UDPTransport) loopIn(conn *dnsConnection) error {
	_buffer := buf.StackNewSize(1024)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	for {
		buffer.FullReset()
		_, err := buffer.ReadFrom(conn)
		if err != nil {
			return err
		}
		var message dnsmessage.Message
		err = message.Unpack(buffer.Bytes())
		if err != nil {
			return err
		}
		conn.access.Lock()
		callback, loaded := conn.callbacks[message.ID]
		if loaded {
			delete(conn.callbacks, message.ID)
		}
		conn.access.Unlock()
		if !loaded {
			continue
		}
		callback <- &message
	}
}

func (t *UDPTransport) Exchange(ctx context.Context, message *dnsmessage.Message) (*dnsmessage.Message, error) {
	var connection *dnsConnection
	err := task.Run(ctx, func() error {
		var innerErr error
		connection, innerErr = t.offer()
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	connection.access.Lock()
	connection.queryId++
	message.ID = connection.queryId
	callback := make(chan *dnsmessage.Message)
	connection.callbacks[message.ID] = callback
	connection.access.Unlock()
	_buffer := buf.StackNewSize(1024)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	rawMessage, err := message.AppendPack(buffer.Index(0))
	if err != nil {
		return nil, err
	}
	buffer.Truncate(len(rawMessage))
	err = task.Run(ctx, func() error {
		return common.Error(connection.Write(buffer.Bytes()))
	})
	if err != nil {
		return nil, err
	}
	select {
	case response := <-callback:
		return response, nil
	case <-connection.done:
		return nil, connection.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
