package hysteria2

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/sing-box/common/baderror"
	"github.com/sagernet/sing-box/common/qtls"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/transport/hysteria2/congestion"
	"github.com/sagernet/sing-box/transport/hysteria2/internal/protocol"
	tuicCongestion "github.com/sagernet/sing-box/transport/tuic/congestion"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type ServerOptions struct {
	Context               context.Context
	Logger                logger.Logger
	SendBPS               uint64
	ReceiveBPS            uint64
	IgnoreClientBandwidth bool
	SalamanderPassword    string
	TLSConfig             tls.ServerConfig
	Users                 []User
	UDPDisabled           bool
	Handler               ServerHandler
	MasqueradeHandler     http.Handler
}

type User struct {
	Name     string
	Password string
}

type ServerHandler interface {
	N.TCPConnectionHandler
	N.UDPConnectionHandler
}

type Server struct {
	ctx                   context.Context
	logger                logger.Logger
	sendBPS               uint64
	receiveBPS            uint64
	ignoreClientBandwidth bool
	salamanderPassword    string
	tlsConfig             tls.ServerConfig
	quicConfig            *quic.Config
	userMap               map[string]User
	udpDisabled           bool
	handler               ServerHandler
	masqueradeHandler     http.Handler
	quicListener          io.Closer
}

func NewServer(options ServerOptions) (*Server, error) {
	quicConfig := &quic.Config{
		DisablePathMTUDiscovery:        !(runtime.GOOS == "windows" || runtime.GOOS == "linux" || runtime.GOOS == "android" || runtime.GOOS == "darwin"),
		MaxDatagramFrameSize:           1400,
		EnableDatagrams:                !options.UDPDisabled,
		MaxIncomingStreams:             1 << 60,
		InitialStreamReceiveWindow:     defaultStreamReceiveWindow,
		MaxStreamReceiveWindow:         defaultStreamReceiveWindow,
		InitialConnectionReceiveWindow: defaultConnReceiveWindow,
		MaxConnectionReceiveWindow:     defaultConnReceiveWindow,
		MaxIdleTimeout:                 defaultMaxIdleTimeout,
		KeepAlivePeriod:                defaultKeepAlivePeriod,
	}
	if len(options.Users) == 0 {
		return nil, E.New("missing users")
	}
	userMap := make(map[string]User)
	for _, user := range options.Users {
		userMap[user.Password] = user
	}
	if options.MasqueradeHandler == nil {
		options.MasqueradeHandler = http.NotFoundHandler()
	}
	return &Server{
		ctx:                   options.Context,
		logger:                options.Logger,
		sendBPS:               options.SendBPS,
		receiveBPS:            options.ReceiveBPS,
		ignoreClientBandwidth: options.IgnoreClientBandwidth,
		salamanderPassword:    options.SalamanderPassword,
		tlsConfig:             options.TLSConfig,
		quicConfig:            quicConfig,
		userMap:               userMap,
		udpDisabled:           options.UDPDisabled,
		handler:               options.Handler,
		masqueradeHandler:     options.MasqueradeHandler,
	}, nil
}

func (s *Server) Start(conn net.PacketConn) error {
	if s.salamanderPassword != "" {
		conn = NewSalamanderConn(conn, []byte(s.salamanderPassword))
	}
	err := qtls.ConfigureHTTP3(s.tlsConfig)
	if err != nil {
		return err
	}
	listener, err := qtls.Listen(conn, s.tlsConfig, s.quicConfig)
	if err != nil {
		return err
	}
	s.quicListener = listener
	go s.loopConnections(listener)
	return nil
}

func (s *Server) Close() error {
	return common.Close(
		s.quicListener,
	)
}

func (s *Server) loopConnections(listener qtls.QUICListener) {
	for {
		connection, err := listener.Accept(s.ctx)
		if err != nil {
			if strings.Contains(err.Error(), "server closed") {
				s.logger.Debug(E.Cause(err, "listener closed"))
			} else {
				s.logger.Error(E.Cause(err, "listener closed"))
			}
			return
		}
		go s.handleConnection(connection)
	}
}

func (s *Server) handleConnection(connection quic.Connection) {
	session := &serverSession{
		Server:     s,
		ctx:        s.ctx,
		quicConn:   connection,
		source:     M.SocksaddrFromNet(connection.RemoteAddr()),
		connDone:   make(chan struct{}),
		udpConnMap: make(map[uint32]*udpPacketConn),
	}
	httpServer := http3.Server{
		Handler:        session,
		StreamHijacker: session.handleStream0,
	}
	_ = httpServer.ServeQUICConn(connection)
	_ = connection.CloseWithError(0, "")
}

type serverSession struct {
	*Server
	ctx           context.Context
	quicConn      quic.Connection
	source        M.Socksaddr
	connAccess    sync.Mutex
	connDone      chan struct{}
	connErr       error
	authenticated bool
	authUser      *User
	udpAccess     sync.RWMutex
	udpConnMap    map[uint32]*udpPacketConn
}

func (s *serverSession) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.Host == protocol.URLHost && r.URL.Path == protocol.URLPath {
		if s.authenticated {
			protocol.AuthResponseToHeader(w.Header(), protocol.AuthResponse{
				UDPEnabled: !s.udpDisabled,
				Rx:         s.receiveBPS,
				RxAuto:     s.ignoreClientBandwidth,
			})
			w.WriteHeader(protocol.StatusAuthOK)
			return
		}
		request := protocol.AuthRequestFromHeader(r.Header)
		user, loaded := s.userMap[request.Auth]
		if !loaded {
			s.masqueradeHandler.ServeHTTP(w, r)
			return
		}
		s.authUser = &user
		s.authenticated = true
		if !s.ignoreClientBandwidth && request.Rx > 0 {
			var sendBps uint64
			if s.sendBPS > 0 && s.sendBPS < request.Rx {
				sendBps = s.sendBPS
			} else {
				sendBps = request.Rx
			}
			s.quicConn.SetCongestionControl(congestion.NewBrutalSender(sendBps))
		} else {
			s.quicConn.SetCongestionControl(tuicCongestion.NewBBRSender(
				tuicCongestion.DefaultClock{},
				tuicCongestion.GetInitialPacketSize(s.quicConn.RemoteAddr()),
				tuicCongestion.InitialCongestionWindow*tuicCongestion.InitialMaxDatagramSize,
				tuicCongestion.DefaultBBRMaxCongestionWindow*tuicCongestion.InitialMaxDatagramSize,
			))
		}
		protocol.AuthResponseToHeader(w.Header(), protocol.AuthResponse{
			UDPEnabled: !s.udpDisabled,
			Rx:         s.receiveBPS,
			RxAuto:     s.ignoreClientBandwidth,
		})
		w.WriteHeader(protocol.StatusAuthOK)
		if s.ctx.Done() != nil {
			go func() {
				select {
				case <-s.ctx.Done():
					s.closeWithError(s.ctx.Err())
				case <-s.connDone:
				}
			}()
		}
		if !s.udpDisabled {
			go s.loopMessages()
		}
	} else {
		s.masqueradeHandler.ServeHTTP(w, r)
	}
}

func (s *serverSession) handleStream0(frameType http3.FrameType, connection quic.Connection, stream quic.Stream, err error) (bool, error) {
	if !s.authenticated || err != nil {
		return false, nil
	}
	if frameType != protocol.FrameTypeTCPRequest {
		return false, nil
	}
	go func() {
		hErr := s.handleStream(stream)
		if hErr != nil {
			stream.CancelRead(0)
			stream.Close()
			s.logger.Error(E.Cause(hErr, "handle stream request"))
		}
	}()
	return true, nil
}

func (s *serverSession) handleStream(stream quic.Stream) error {
	destinationString, err := protocol.ReadTCPRequest(stream)
	if err != nil {
		return E.New("read TCP request")
	}
	var conn net.Conn = &serverConn{
		Stream: stream,
	}
	ctx := s.ctx
	if s.authUser.Name != "" {
		ctx = auth.ContextWithUser(s.ctx, s.authUser.Name)
	}
	_ = s.handler.NewConnection(ctx, conn, M.Metadata{
		Source:      s.source,
		Destination: M.ParseSocksaddr(destinationString),
	})
	return nil
}

func (s *serverSession) closeWithError(err error) {
	s.connAccess.Lock()
	defer s.connAccess.Unlock()
	select {
	case <-s.connDone:
		return
	default:
		s.connErr = err
		close(s.connDone)
	}
	if E.IsClosedOrCanceled(err) {
		s.logger.Debug(E.Cause(err, "connection failed"))
	} else {
		s.logger.Error(E.Cause(err, "connection failed"))
	}
	_ = s.quicConn.CloseWithError(0, "")
}

type serverConn struct {
	quic.Stream
	responseWritten bool
}

func (c *serverConn) HandshakeFailure(err error) error {
	if c.responseWritten {
		return os.ErrClosed
	}
	c.responseWritten = true
	buffer := protocol.WriteTCPResponse(false, err.Error(), nil)
	defer buffer.Release()
	return common.Error(c.Stream.Write(buffer.Bytes()))
}

func (c *serverConn) Read(p []byte) (n int, err error) {
	n, err = c.Stream.Read(p)
	return n, baderror.WrapQUIC(err)
}

func (c *serverConn) Write(p []byte) (n int, err error) {
	if !c.responseWritten {
		c.responseWritten = true
		buffer := protocol.WriteTCPResponse(true, "", p)
		defer buffer.Release()
		_, err = c.Stream.Write(buffer.Bytes())
		if err != nil {
			return 0, baderror.WrapQUIC(err)
		}
		return len(p), nil
	}
	n, err = c.Stream.Write(p)
	return n, baderror.WrapQUIC(err)
}

func (c *serverConn) LocalAddr() net.Addr {
	return M.Socksaddr{}
}

func (c *serverConn) RemoteAddr() net.Addr {
	return M.Socksaddr{}
}

func (c *serverConn) Close() error {
	c.Stream.CancelRead(0)
	return c.Stream.Close()
}
