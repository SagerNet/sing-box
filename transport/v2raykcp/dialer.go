package v2raykcp

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	ctx       context.Context
	dialer    N.Dialer
	serverAddr M.Socksaddr
	config    *Config
	tlsConfig tls.Config
}

func NewClient(
	ctx context.Context,
	dialer N.Dialer,
	serverAddr M.Socksaddr,
	options option.V2RayKCPOptions,
	tlsConfig tls.Config,
) (adapter.V2RayClientTransport, error) {
	return &Client{
		ctx:        ctx,
		dialer:     dialer,
		serverAddr: serverAddr,
		config:     NewConfig(options),
		tlsConfig:  tlsConfig,
	}, nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	// Dial UDP connection
	udpConn, err := c.dialer.DialContext(ctx, N.NetworkUDP, c.serverAddr)
	if err != nil {
		return nil, E.Cause(err, "dial UDP")
	}

	// Wrap as PacketConn
	packetConn := bufio.NewUnbindPacketConn(udpConn)

	// Generate conversation ID
	var convID uint16
	binary.Read(rand.Reader, binary.BigEndian, &convID)

	// Create KCP connection
	kcpConn, err := c.createConnection(ctx, packetConn, c.serverAddr.UDPAddr(), convID)
	if err != nil {
		udpConn.Close()
		return nil, E.Cause(err, "create KCP connection")
	}

	// Wrap with TLS if configured
	if c.tlsConfig != nil {
		tlsConn, err := tls.ClientHandshake(ctx, kcpConn, c.tlsConfig)
		if err != nil {
			kcpConn.Close()
			return nil, E.Cause(err, "TLS handshake")
		}
		return tlsConn, nil
	}

	return kcpConn, nil
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) createConnection(ctx context.Context, conn N.PacketConn, remoteAddr *net.UDPAddr, convID uint16) (*Connection, error) {
	security, err := c.config.GetSecurity()
	if err != nil {
		return nil, E.Cause(err, "get security")
	}

	// Create packet header
	header := c.config.GetPacketHeader()

	// Create packet writer
	writer := &kcpPacketWriter{
		conn:       conn,
		remoteAddr: remoteAddr,
		header:     header,
		security:   security,
	}

	// Create packet reader
	reader := &kcpPacketReader{
		security:   security,
		headerSize: HeaderSize(c.config.GetHeaderType()),
	}

	// Create connection metadata
	meta := ConnMetadata{
		LocalAddr:    conn.LocalAddr(),
		RemoteAddr:   remoteAddr,
		Conversation: convID,
	}

	// Create KCP connection
	kcpConn := NewConnection(meta, writer, conn, c.config)

	// Start reading goroutine
	go c.readLoop(ctx, conn, reader, kcpConn)

	return kcpConn, nil
}

func (c *Client) readLoop(ctx context.Context, conn N.PacketConn, reader *kcpPacketReader, kcpConn *Connection) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		buffer := buf.New()
		_, err := conn.ReadPacket(buffer)
		if err != nil {
			buffer.Release()
			return
		}

		segments := reader.Read(buffer.Bytes())
		buffer.Release()
		
		if len(segments) > 0 {
			kcpConn.Input(segments)
		}
	}
}

type kcpPacketWriter struct {
	conn       N.PacketConn
	remoteAddr *net.UDPAddr
	header     PacketHeader
	security   cipher.AEAD
}

func (w *kcpPacketWriter) Overhead() int {
	overhead := 0
	if w.header != nil {
		overhead += w.header.Size()
	}
	if w.security != nil {
		overhead += w.security.Overhead()
	}
	return overhead
}

func (w *kcpPacketWriter) Write(b []byte) (int, error) {
	packet := buf.New()
	defer packet.Release()

	if w.header != nil {
		headerBytes := packet.Extend(w.header.Size())
		w.header.Serialize(headerBytes)
	}

	if w.security != nil {
		nonceSize := w.security.NonceSize()
		nonce := packet.Extend(nonceSize)
		common.Must1(rand.Read(nonce))

		encrypted := w.security.Seal(nil, nonce, b, nil)
		packet.Write(encrypted)
	} else {
		packet.Write(b)
	}

	destAddr := M.SocksaddrFromNet(w.remoteAddr)
	err := w.conn.WritePacket(packet, destAddr)
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

type kcpPacketReader struct {
	security   cipher.AEAD
	headerSize int
}

func (r *kcpPacketReader) Read(b []byte) []Segment {
	if r.headerSize > 0 {
		if len(b) <= r.headerSize {
			return nil
		}
		b = b[r.headerSize:]
	}

	if r.security != nil {
		nonceSize := r.security.NonceSize()
		overhead := r.security.Overhead()
		if len(b) <= nonceSize+overhead {
			return nil
		}
		out, err := r.security.Open(nil, b[:nonceSize], b[nonceSize:], nil)
		if err != nil {
			return nil
		}
		b = out
	}

	var result []Segment
	for len(b) > 0 {
		seg, extra := ReadSegment(b)
		if seg == nil {
			break
		}
		result = append(result, seg)
		b = extra
	}
	return result
}
