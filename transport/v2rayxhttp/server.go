package v2rayxhttp

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"
	sHTTP "github.com/sagernet/sing/protocol/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx          context.Context
	logger       logger.ContextLogger
	tlsConfig    tls.ServerConfig
	handler      adapter.V2RayServerTransportHandler
	config       *config
	httpServer   *http.Server
	packetServer io.Closer
	sessions     sync.Map
	// sessionTimeout bounds an upload-only session that never receives its
	// paired download request. It is a field rather than a package constant so
	// lifecycle tests can exercise cleanup without waiting for the production
	// timeout.
	sessionTimeout time.Duration
}

func NewServer(ctx context.Context, logger logger.ContextLogger, options option.V2RayXHTTPOptions, tlsConfig tls.ServerConfig, handler adapter.V2RayServerTransportHandler) (*Server, error) {
	config, err := newConfig(options)
	if err != nil {
		return nil, err
	}
	if isHTTP3(tlsConfig) {
		if err = validateHTTP3(); err != nil {
			return nil, err
		}
	}
	server := &Server{ctx: ctx, logger: logger, tlsConfig: tlsConfig, handler: handler, config: config, sessionTimeout: 30 * time.Second}
	handlerWithH2C := h2c.NewHandler(server, &http2.Server{})
	server.httpServer = &http.Server{
		Handler:           handlerWithH2C,
		ReadHeaderTimeout: C.TCPTimeout,
		MaxHeaderBytes:    config.maxHeaderBytes,
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ConnContext:       func(ctx context.Context, conn net.Conn) context.Context { return log.ContextWithNewID(ctx) },
	}
	return server, nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if s.config.host != "" && request.Host != s.config.host {
		s.invalid(writer, request, http.StatusNotFound, E.New("bad host"))
		return
	}
	if !strings.HasPrefix(request.URL.Path, s.config.path) {
		s.invalid(writer, request, http.StatusNotFound, E.New("bad path"))
		return
	}
	s.writeCORS(writer, request)
	s.config.applyResponsePadding(writer)
	if request.Method == http.MethodOptions {
		writer.WriteHeader(http.StatusOK)
		return
	}
	if !s.config.validPadding(request) {
		s.invalid(writer, request, http.StatusBadRequest, E.New("invalid xhttp padding"))
		return
	}
	sessionID, sequence := s.config.extractMetadata(request)
	if sessionID == "" && s.config.mode != "auto" && s.config.mode != "stream-one" {
		s.invalid(writer, request, http.StatusBadRequest, E.New("stream-one is not allowed"))
		return
	}

	if isUplink(request, sequence) && sessionID != "" {
		session := s.session(sessionID)
		if sequence == "" {
			if s.config.mode != "auto" && s.config.mode != "stream-up" {
				s.invalid(writer, request, http.StatusBadRequest, E.New("stream-up is not allowed"))
				return
			}
			session.startStream(request.Body)
			_ = http.NewResponseController(writer).EnableFullDuplex()
			writer.Header().Set("X-Accel-Buffering", "no")
			writer.Header().Set("Cache-Control", "no-store")
			writer.WriteHeader(http.StatusOK)
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}
			if s.keepStreamUpResponseAlive(writer, request) {
				return
			}
			<-request.Context().Done()
			return
		}
		if s.config.mode != "auto" && s.config.mode != "packet-up" {
			s.invalid(writer, request, http.StatusBadRequest, E.New("packet-up is not allowed"))
			return
		}
		payload, err := s.config.extractPacketPayload(request)
		if err != nil || len(payload) > s.config.scMaxPost.random() {
			s.invalid(writer, request, http.StatusBadRequest, E.New("invalid xhttp upload"))
			return
		}
		seq, err := strconv.ParseUint(sequence, 10, 64)
		if err != nil {
			s.invalid(writer, request, http.StatusBadRequest, E.New("invalid xhttp sequence"))
			return
		}
		if !session.push(packet{sequence: seq, payload: payload}) {
			s.invalid(writer, request, http.StatusConflict, E.New("closed xhttp session"))
			return
		}
		writer.Header().Set("Cache-Control", "no-store")
		writer.WriteHeader(http.StatusOK)
		return
	}

	if request.Method != http.MethodGet && sessionID != "" {
		s.invalid(writer, request, http.StatusMethodNotAllowed, E.New("unsupported xhttp method"))
		return
	}
	// Go's HTTP/1.1 server otherwise drains a request body before flushing the
	// response. XHTTP stream-one deliberately needs both directions live.
	if request.Body != nil {
		_ = http.NewResponseController(writer).EnableFullDuplex()
	}
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.Header().Set("Cache-Control", "no-store")
	if !s.config.noSSEHeader {
		writer.Header().Set("Content-Type", "text/event-stream")
	}
	writer.WriteHeader(http.StatusOK)
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}

	reader := io.Reader(request.Body)
	var session *serverSession
	if sessionID != "" {
		session = s.session(sessionID)
		session.markConnected()
		reader = session.reader
	}
	response := newResponseConn(reader, writer, request.Context().Done())
	defer response.Close()
	if session != nil {
		defer s.closeSession(sessionID, session)
	}
	done := make(chan struct{})
	var closeOnce sync.Once
	onClose := func(error) { closeOnce.Do(func() { close(done) }) }
	s.handler.NewConnectionEx(request.Context(), response, sHTTP.SourceAddress(request), M.Socksaddr{}, onClose)
	select {
	case <-done:
	case <-request.Context().Done():
	}
}

func (s *Server) keepStreamUpResponseAlive(writer http.ResponseWriter, request *http.Request) bool {
	if s.config.scStreamUp.to <= 0 || !(request.Header.Get("Referer") != "" || s.config.paddingObfs) {
		return false
	}
	for {
		if _, err := io.WriteString(writer, strings.Repeat("X", s.config.padding.random())); err != nil {
			return true
		}
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
		timer := time.NewTimer(time.Duration(s.config.scStreamUp.random()) * time.Second)
		select {
		case <-request.Context().Done():
			timer.Stop()
			return true
		case <-timer.C:
		}
	}
}

func isUplink(request *http.Request, sequence string) bool {
	return request.Method != http.MethodGet || sequence != ""
}

func (s *Server) session(id string) *serverSession {
	if existing, loaded := s.sessions.Load(id); loaded {
		return existing.(*serverSession)
	}
	session := newServerSession(s.config.maxBufferedPosts)
	actual, loaded := s.sessions.LoadOrStore(id, session)
	if loaded {
		session.close()
		return actual.(*serverSession)
	}
	go func() {
		select {
		case <-time.After(s.sessionTimeout):
			s.closeSession(id, session)
		case <-session.connected:
		}
	}()
	return session
}

func (s *Server) closeSession(id string, session *serverSession) {
	session.markConnected()
	s.sessions.CompareAndDelete(id, session)
	session.close()
}

func (s *Server) writeCORS(writer http.ResponseWriter, request *http.Request) {
	origin := request.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	writer.Header().Set("Access-Control-Allow-Origin", origin)
	if s.config.sessionPlacement == placementCookie || s.config.seqPlacement == placementCookie || s.config.dataPlacement == placementCookie || s.config.paddingPlacement == placementCookie {
		writer.Header().Set("Access-Control-Allow-Credentials", "true")
	}
}

func (s *Server) invalid(writer http.ResponseWriter, request *http.Request, status int, err error) {
	writer.WriteHeader(status)
	s.logger.DebugContext(request.Context(), E.Cause(err, "process xhttp request from ", request.RemoteAddr))
}
func (s *Server) Network() []string {
	if isHTTP3(s.tlsConfig) {
		return []string{N.NetworkUDP}
	}
	return []string{N.NetworkTCP}
}
func (s *Server) Serve(listener net.Listener) error {
	if isHTTP3(s.tlsConfig) {
		return os.ErrInvalid
	}
	if s.tlsConfig != nil {
		if len(s.tlsConfig.NextProtos()) == 0 {
			s.tlsConfig.SetNextProtos([]string{http2.NextProtoTLS, "http/1.1"})
		}
		listener = aTLS.NewListener(listener, s.tlsConfig)
	}
	return s.httpServer.Serve(listener)
}
func (s *Server) Close() error {
	s.sessions.Range(func(_, value any) bool { value.(*serverSession).close(); return true })
	return common.Close(s.httpServer, s.packetServer)
}

type packet struct {
	sequence uint64
	payload  []byte
}
type serverSession struct {
	reader      *io.PipeReader
	writer      *io.PipeWriter
	packets     chan packet
	closed      chan struct{}
	connected   chan struct{}
	closeOnce   sync.Once
	connectOnce sync.Once
}

func newServerSession(maxPackets int) *serverSession {
	reader, writer := io.Pipe()
	session := &serverSession{reader: reader, writer: writer, packets: make(chan packet, maxPackets), closed: make(chan struct{}), connected: make(chan struct{})}
	go session.copyPackets()
	return session
}
func (s *serverSession) copyPackets() {
	pending := make(map[uint64][]byte)
	var next uint64
	for {
		select {
		case item := <-s.packets:
			pending[item.sequence] = item.payload
			for {
				payload, ok := pending[next]
				if !ok {
					break
				}
				delete(pending, next)
				if _, err := s.writer.Write(payload); err != nil {
					return
				}
				next++
			}
		case <-s.closed:
			return
		}
	}
}
func (s *serverSession) push(item packet) bool {
	select {
	case s.packets <- item:
		return true
	case <-s.closed:
		return false
	}
}
func (s *serverSession) startStream(reader io.ReadCloser) {
	go func() { _, _ = io.Copy(s.writer, reader); _ = reader.Close() }()
}
func (s *serverSession) markConnected() { s.connectOnce.Do(func() { close(s.connected) }) }
func (s *serverSession) close() {
	s.closeOnce.Do(func() { close(s.closed); _ = s.reader.Close(); _ = s.writer.Close(); s.markConnected() })
}
