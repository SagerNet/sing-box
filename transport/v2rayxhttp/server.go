package v2rayxhttp

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
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
	sHttp "github.com/sagernet/sing/protocol/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

// Session represents a client session with upload and download streams
type Session struct {
	uploadQueue     *uploadQueue
	downloadWriter  io.WriteCloser
	downloadReader  io.ReadCloser
	isConnected     bool
	createdAt       time.Time
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
}

type Server struct {
	ctx                context.Context
	logger             logger.ContextLogger
	tlsConfig          tls.ServerConfig
	handler            adapter.V2RayServerTransportHandler
	httpServer         *http.Server
	h2Server           *http2.Server
	h2cHandler         http.Handler
	host               string
	path               string
	headers            http.Header
	noSSEHeader        bool
	xPaddingBytes      *option.RangeConfig
	scMaxBufferedPosts int
	sessions           sync.Map // map[string]*Session
	sessionPathRegex   *regexp.Regexp
}

func NewServer(ctx context.Context, logger logger.ContextLogger, options option.V2RayXHTTPOptions, tlsConfig tls.ServerConfig, handler adapter.V2RayServerTransportHandler) (*Server, error) {
	path := options.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	scMaxBufferedPosts := options.ScMaxBufferedPosts
	if scMaxBufferedPosts <= 0 {
		scMaxBufferedPosts = 30
	}

	server := &Server{
		ctx:                ctx,
		logger:             logger,
		tlsConfig:          tlsConfig,
		handler:            handler,
		h2Server:           &http2.Server{},
		host:               options.Host,
		path:               path,
		headers:            options.Headers.Build(),
		noSSEHeader:        options.NoSSEHeader,
		xPaddingBytes:      options.XPaddingBytes,
		scMaxBufferedPosts: scMaxBufferedPosts,
		sessionPathRegex:   regexp.MustCompile(`^` + regexp.QuoteMeta(path) + `([a-f0-9]{32})(?:/(\d+))?$`),
	}

	server.httpServer = &http.Server{
		Handler:           server,
		ReadHeaderTimeout: C.TCPTimeout,
		MaxHeaderBytes:    http.DefaultMaxHeaderBytes,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			return log.ContextWithNewID(ctx)
		},
	}
	server.h2cHandler = h2c.NewHandler(server, server.h2Server)

	return server, nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	// Handle h2c upgrade
	if request.Method == "PRI" && len(request.Header) == 0 && request.URL.Path == "*" && request.Proto == "HTTP/2.0" {
		s.h2cHandler.ServeHTTP(writer, request)
		return
	}

	// Validate host
	if s.host != "" && request.Host != s.host {
		s.invalidRequest(writer, request, http.StatusBadRequest, E.New("bad host: ", request.Host))
		return
	}

	// Validate path
	requestPath := request.URL.Path
	if !strings.HasPrefix(requestPath, s.path) {
		s.invalidRequest(writer, request, http.StatusNotFound, E.New("bad path: ", requestPath))
		return
	}

	// Parse session ID and sequence from path
	matches := s.sessionPathRegex.FindStringSubmatch(requestPath)

	// Handle stream-one mode (POST to base path)
	if requestPath == s.path && request.Method == http.MethodPost {
		s.handleStreamOne(writer, request)
		return
	}

	if matches == nil {
		s.invalidRequest(writer, request, http.StatusNotFound, E.New("invalid path format: ", requestPath))
		return
	}

	sessionID := matches[1]
	seqStr := matches[2]

	switch request.Method {
	case http.MethodGet:
		s.handleDownload(writer, request, sessionID)
	case http.MethodPost:
		if seqStr != "" {
			// packet-up mode
			seq, err := strconv.Atoi(seqStr)
			if err != nil {
				s.invalidRequest(writer, request, http.StatusBadRequest, E.New("invalid sequence: ", seqStr))
				return
			}
			s.handlePacketUpload(writer, request, sessionID, seq)
		} else {
			// stream-up mode
			s.handleStreamUpload(writer, request, sessionID)
		}
	default:
		s.invalidRequest(writer, request, http.StatusMethodNotAllowed, E.New("unsupported method: ", request.Method))
	}
}

func (s *Server) handleStreamOne(writer http.ResponseWriter, request *http.Request) {
	source := sHttp.SourceAddress(request)

	// Set response headers
	s.setResponseHeaders(writer)
	writer.WriteHeader(http.StatusOK)

	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}

	// Create bidirectional connection
	conn := newHTTPConn(request.Body, writer)

	s.handler.NewConnectionEx(request.Context(), conn, source, M.Socksaddr{}, nil)
}

func (s *Server) handleDownload(writer http.ResponseWriter, request *http.Request, sessionID string) {
	session := s.getOrCreateSession(sessionID)

	session.mu.Lock()
	if session.isConnected {
		session.mu.Unlock()
		s.invalidRequest(writer, request, http.StatusConflict, E.New("session already has download connection"))
		return
	}
	session.isConnected = true
	session.mu.Unlock()

	// Set response headers
	s.setResponseHeaders(writer)
	writer.WriteHeader(http.StatusOK)

	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}

	// Create connection with upload queue as reader and response writer as writer
	source := sHttp.SourceAddress(request)
	conn := newHTTPConn(session.uploadQueue, &responseWriter{writer, request.Context()})

	// Handle connection
	done := make(chan struct{})
	s.handler.NewConnectionEx(request.Context(), conn, source, M.Socksaddr{}, N.OnceClose(func(it error) {
		close(done)
	}))

	<-done

	// Cleanup session
	s.sessions.Delete(sessionID)
	session.cancel()
}

func (s *Server) handleStreamUpload(writer http.ResponseWriter, request *http.Request, sessionID string) {
	session := s.getOrCreateSession(sessionID)

	// Read all data from request body and write to upload queue
	go func() {
		io.Copy(session.uploadQueue, request.Body)
		request.Body.Close()
	}()

	s.setResponseHeaders(writer)
	writer.WriteHeader(http.StatusOK)
}

func (s *Server) handlePacketUpload(writer http.ResponseWriter, request *http.Request, sessionID string, seq int) {
	session := s.getOrCreateSession(sessionID)

	// Read packet data
	data, err := io.ReadAll(request.Body)
	request.Body.Close()
	if err != nil {
		s.invalidRequest(writer, request, http.StatusInternalServerError, E.Cause(err, "read request body"))
		return
	}

	// Add to upload queue with sequence number
	session.uploadQueue.Push(seq, data)

	s.setResponseHeaders(writer)
	writer.WriteHeader(http.StatusOK)
}

func (s *Server) getOrCreateSession(sessionID string) *Session {
	if session, ok := s.sessions.Load(sessionID); ok {
		return session.(*Session)
	}

	ctx, cancel := context.WithCancel(s.ctx)
	session := &Session{
		uploadQueue: newUploadQueue(s.scMaxBufferedPosts),
		createdAt:   time.Now(),
		ctx:         ctx,
		cancel:      cancel,
	}

	actual, loaded := s.sessions.LoadOrStore(sessionID, session)
	if loaded {
		cancel()
		return actual.(*Session)
	}

	// Start cleanup timer
	go func() {
		select {
		case <-time.After(30 * time.Second):
			session.mu.Lock()
			connected := session.isConnected
			session.mu.Unlock()
			if !connected {
				s.sessions.Delete(sessionID)
				session.uploadQueue.Close()
				cancel()
			}
		case <-ctx.Done():
		}
	}()

	return session
}

func (s *Server) setResponseHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Accel-Buffering", "no")

	if !s.noSSEHeader {
		writer.Header().Set("Content-Type", "text/event-stream")
	}

	// Add padding
	padding := s.generatePadding()
	writer.Header().Set("X-Padding", padding)

	// Add custom headers
	for key, values := range s.headers {
		for _, value := range values {
			writer.Header().Set(key, value)
		}
	}
}

func (s *Server) generatePadding() string {
	minBytes := int32(100)
	maxBytes := int32(1000)
	if s.xPaddingBytes != nil {
		minBytes = s.xPaddingBytes.GetFrom(100)
		maxBytes = s.xPaddingBytes.GetTo(1000)
	}
	length := minBytes
	if maxBytes > minBytes {
		length = minBytes + s.xPaddingBytes.RandValue()%(maxBytes-minBytes+1)
	}
	padding := make([]byte, length)
	for i := range padding {
		padding[i] = 'A' + byte(i%26)
	}
	return string(padding)
}

func (s *Server) invalidRequest(writer http.ResponseWriter, request *http.Request, statusCode int, err error) {
	if statusCode > 0 {
		writer.WriteHeader(statusCode)
	}
	s.logger.ErrorContext(request.Context(), E.Cause(err, "process connection from ", request.RemoteAddr))
}

func (s *Server) Network() []string {
	return []string{N.NetworkTCP}
}

func (s *Server) Serve(listener net.Listener) error {
	if s.tlsConfig != nil {
		if len(s.tlsConfig.NextProtos()) == 0 {
			s.tlsConfig.SetNextProtos([]string{http2.NextProtoTLS, "http/1.1"})
		} else if !common.Contains(s.tlsConfig.NextProtos(), http2.NextProtoTLS) {
			s.tlsConfig.SetNextProtos(append([]string{http2.NextProtoTLS}, s.tlsConfig.NextProtos()...))
		}
		listener = aTLS.NewListener(listener, s.tlsConfig)
	}
	return s.httpServer.Serve(listener)
}

func (s *Server) ServePacket(listener net.PacketConn) error {
	return os.ErrInvalid
}

func (s *Server) Close() error {
	return common.Close(common.PtrOrNil(s.httpServer))
}

// httpConn wraps reader and writer as net.Conn
type httpConn struct {
	reader io.Reader
	writer io.Writer
}

func newHTTPConn(reader io.Reader, writer io.Writer) *httpConn {
	return &httpConn{
		reader: reader,
		writer: writer,
	}
}

func (c *httpConn) Read(b []byte) (n int, err error) {
	return c.reader.Read(b)
}

func (c *httpConn) Write(b []byte) (n int, err error) {
	n, err = c.writer.Write(b)
	if err != nil {
		return
	}
	if flusher, ok := c.writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return
}

func (c *httpConn) Close() error {
	if closer, ok := c.reader.(io.Closer); ok {
		closer.Close()
	}
	if closer, ok := c.writer.(io.Closer); ok {
		closer.Close()
	}
	return nil
}

func (c *httpConn) LocalAddr() net.Addr {
	return dummyAddr{network: "tcp", address: "0.0.0.0:0"}
}

func (c *httpConn) RemoteAddr() net.Addr {
	return dummyAddr{network: "tcp", address: "0.0.0.0:0"}
}

func (c *httpConn) SetDeadline(t time.Time) error      { return nil }
func (c *httpConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *httpConn) SetWriteDeadline(t time.Time) error { return nil }

// responseWriter wraps http.ResponseWriter with context
type responseWriter struct {
	http.ResponseWriter
	ctx context.Context
}

func (w *responseWriter) Write(b []byte) (n int, err error) {
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	default:
		return w.ResponseWriter.Write(b)
	}
}
