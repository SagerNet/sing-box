package v2raykcp

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"net"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/logger"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx       context.Context
	logger    logger.ContextLogger
	config    *Config
	tlsConfig tls.ServerConfig
	handler   adapter.V2RayServerTransportHandler
	listener  *net.UDPConn
	sessions  sync.Map // map[ConnectionID]*Connection
	security  cipher.AEAD
	headerSize int
}

type ConnectionID struct {
	Remote string
	Port   uint16
	Conv   uint16
}

func NewServer(
	ctx context.Context,
	logger logger.ContextLogger,
	options option.V2RayKCPOptions,
	tlsConfig tls.ServerConfig,
	handler adapter.V2RayServerTransportHandler,
) (adapter.V2RayServerTransport, error) {
	config := NewConfig(options)
	security, err := config.GetSecurity()
	if err != nil {
		return nil, E.Cause(err, "get security")
	}

	return &Server{
		ctx:        ctx,
		logger:     logger,
		config:     config,
		tlsConfig:  tlsConfig,
		handler:    handler,
		security:   security,
		headerSize: HeaderSize(config.GetHeaderType()),
	}, nil
}

func (s *Server) Network() []string {
	return []string{N.NetworkUDP}
}

func (s *Server) Serve(listener net.Listener) error {
	return E.New("KCP server requires ServePacket")
}

func (s *Server) ServePacket(listener net.PacketConn) error {
	udpConn, ok := listener.(*net.UDPConn)
	if !ok {
		return E.New("KCP requires UDP listener")
	}

	s.listener = udpConn
	s.logger.Info("KCP server started")

	buffer := make([]byte, 2048)
	for {
		n, remoteAddr, err := udpConn.ReadFrom(buffer)
		if err != nil {
			if E.IsClosed(err) {
				return nil
			}
			return err
		}

		go s.handlePacket(buffer[:n], remoteAddr)
	}
}

func (s *Server) handlePacket(data []byte, remoteAddr net.Addr) {
	reader := &kcpPacketReader{
		security:   s.security,
		headerSize: s.headerSize,
	}

	segments := reader.Read(data)
	if len(segments) == 0 {
		return
	}

	firstSeg := segments[0]
	conv := firstSeg.Conversation()
	cmd := firstSeg.Command()

	udpAddr, ok := remoteAddr.(*net.UDPAddr)
	if !ok {
		return
	}

	connID := ConnectionID{
		Remote: udpAddr.IP.String(),
		Port:   uint16(udpAddr.Port),
		Conv:   conv,
	}

	value, exists := s.sessions.Load(connID)
	if !exists {
		if cmd == CommandTerminate {
			return
		}

		// Create new connection
		writer := &serverPacketWriter{
			conn:       s.listener,
			remoteAddr: udpAddr,
			server:     s,
			connID:     connID,
			header:     s.config.GetPacketHeader(),
			security:   s.security,
		}

		meta := ConnMetadata{
			LocalAddr:    s.listener.LocalAddr(),
			RemoteAddr:   udpAddr,
			Conversation: conv,
		}

		kcpConn := NewConnection(meta, writer, writer, s.config)
		s.sessions.Store(connID, kcpConn)

		var netConn net.Conn = kcpConn
		if s.tlsConfig != nil {
			tlsConn, err := tls.ServerHandshake(s.ctx, kcpConn, s.tlsConfig)
			if err != nil {
				kcpConn.Close()
				s.sessions.Delete(connID)
				return
			}
			netConn = tlsConn
		}

		source := M.SocksaddrFromNet(remoteAddr)
		go s.handler.NewConnectionEx(s.ctx, netConn, source, M.Socksaddr{}, nil)

		kcpConn.Input(segments)
	} else {
		conn := value.(*Connection)
		conn.Input(segments)
	}
}

func (s *Server) Close() error {
	s.sessions.Range(func(key, value interface{}) bool {
		conn := value.(*Connection)
		conn.Close()
		return true
	})
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

type serverPacketWriter struct {
	conn       *net.UDPConn
	remoteAddr *net.UDPAddr
	server     *Server
	connID     ConnectionID
	header     PacketHeader
	security   cipher.AEAD
}

func (w *serverPacketWriter) Overhead() int {
	overhead := 0
	if w.header != nil {
		overhead += w.header.Size()
	}
	if w.security != nil {
		overhead += w.security.Overhead()
	}
	return overhead
}

func (w *serverPacketWriter) Write(b []byte) (int, error) {
	buffer := buf.New()
	defer buffer.Release()

	if w.header != nil {
		headerBytes := buffer.Extend(w.header.Size())
		w.header.Serialize(headerBytes)
	}

	if w.security != nil {
		nonceSize := w.security.NonceSize()
		nonce := buffer.Extend(nonceSize)
		common.Must1(rand.Read(nonce))

		encrypted := w.security.Seal(nil, nonce, b, nil)
		buffer.Write(encrypted)
	} else {
		buffer.Write(b)
	}

	_, err := w.conn.WriteTo(buffer.Bytes(), w.remoteAddr)
	return len(b), err
}

func (w *serverPacketWriter) Close() error {
	w.server.sessions.Delete(w.connID)
	return nil
}
