package wsc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"
)

var _ network.NetPacketReader = &clientPacketConn{}
var _ network.NetPacketWriter = &clientPacketConn{}

type readerCache struct {
	reader *bytes.Reader
	addr   metadata.Socksaddr
}

type clientPacketConn struct {
	net.Conn
	reader *wsutil.Reader
	cache  *readerCache
	mu     sync.Mutex
}

func (cli *Client) newPacketConn(ctx context.Context, network string, endpoint string) (*clientPacketConn, error) {
	conn, err := cli.newWSConn(ctx, network, endpoint)
	if err != nil {
		return nil, err
	}
	reader := wsutil.NewReader(conn, ws.StateClientSide)
	return &clientPacketConn{
		Conn:   conn,
		reader: reader,
		cache:  nil,
	}, nil
}

func (packetConn *clientPacketConn) ReadPacket(buffer *buf.Buffer) (destination metadata.Socksaddr, err error) {
	if buffer == nil {
		return metadata.Socksaddr{}, errors.New("buffer is nil")
	}

	buf, err := wsutil.ReadServerBinary(packetConn.Conn)
	if err != nil {
		var cerr wsutil.ClosedError
		if errors.Is(err, &cerr) {
			return metadata.Socksaddr{}, err
		}
		return metadata.Socksaddr{}, err
	}

	payload := packetConnPayload{}
	if err := payload.UnmarshalBinaryUnsafe(buf); err != nil {
		return metadata.Socksaddr{}, err
	}

	destination = metadata.SocksaddrFromNetIP(payload.addrPort)

	if _, err := buffer.Write(payload.payload); err != nil {
		return metadata.Socksaddr{}, err
	}

	return destination, nil
}

func (packetConn *clientPacketConn) WritePacket(buffer *buf.Buffer, destination metadata.Socksaddr) error {
	if buffer == nil {
		return errors.New("buffer is nil")
	}

	payload := packetConnPayload{
		addrPort: destination.AddrPort(),
		payload:  buffer.Bytes(),
	}
	payloadBytes, err := payload.MarshalBinary()
	if err != nil {
		return err
	}

	packetConn.mu.Lock()
	defer packetConn.mu.Unlock()

	if err := wsutil.WriteClientBinary(packetConn.Conn, payloadBytes); err != nil {
		return err
	}

	return nil
}

func (packetConn *clientPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	err = nil
	if packetConn.cache != nil {
		n, err = packetConn.cache.reader.Read(p)
		addr = packetConn.cache.addr
		if err == io.EOF {
			err = nil
			packetConn.cache = nil
		} else {
			return
		}
	}

	buf, err := wsutil.ReadServerBinary(packetConn.Conn)
	if err != nil {
		var cerr wsutil.ClosedError
		if errors.Is(err, &cerr) {
			return 0, nil, io.EOF
		}
		return 0, nil, err
	}

	payload := packetConnPayload{}
	if err := payload.UnmarshalBinaryUnsafe(buf); err != nil {
		return 0, nil, err
	}

	packetConn.cache = &readerCache{
		reader: bytes.NewReader(payload.payload),
		addr:   metadata.SocksaddrFromNetIP(payload.addrPort),
	}

	n, err = packetConn.cache.reader.Read(p)
	addr = packetConn.cache.addr
	if err == io.EOF {
		packetConn.cache = nil
	}

	return
}

func (packetConn *clientPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	payload := packetConnPayload{
		addrPort: metadata.SocksaddrFromNet(addr).AddrPort(),
		payload:  p,
	}
	payloadBytes, err := payload.MarshalBinary()
	if err != nil {
		return 0, err
	}

	packetConn.mu.Lock()
	defer packetConn.mu.Unlock()

	if err := wsutil.WriteClientBinary(packetConn.Conn, payloadBytes); err != nil {
		return 0, err
	}

	return len(payloadBytes), nil
}

func (packetConn *clientPacketConn) Close() error {
	packetConn.mu.Lock()
	defer packetConn.mu.Unlock()
	_ = wsutil.WriteClientMessage(packetConn.Conn, ws.OpClose, nil)
	return packetConn.Conn.Close()
}
