package xhttp

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	qtls "github.com/sagernet/sing-quic"

	// qtls "github.com/sagernet/sing-quic"
	"github.com/sagernet/sing-box/common/xray/signal/done"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"
	sHttp "github.com/sagernet/sing/protocol/http"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx         context.Context
	logger      logger.ContextLogger
	tlsConfig   tls.ServerConfig
	quicConfig  *quic.Config
	handler     adapter.V2RayServerTransportHandler
	httpServer  *http.Server
	http3Server *http3.Server
	localAddr   net.Addr
	options     *option.V2RayXHTTPOptions
	host        string
	path        string
	sessionMu   sync.Mutex
	sessions    sync.Map
}

func NewServer(ctx context.Context, logger logger.ContextLogger, options option.V2RayXHTTPOptions, tlsConfig tls.ServerConfig, handler adapter.V2RayServerTransportHandler) (*Server, error) {
	server := &Server{
		ctx:       ctx,
		logger:    logger,
		tlsConfig: tlsConfig,
		handler:   handler,
		options:   &options,
		host:      options.Host,
		path:      options.GetNormalizedPath(),
	}
	if server.network() == N.NetworkTCP {
		protocols := new(http.Protocols)
		protocols.SetHTTP1(true)
		protocols.SetUnencryptedHTTP2(true)
		server.httpServer = &http.Server{
			Handler:           server,
			ReadHeaderTimeout: time.Second * 4,
			MaxHeaderBytes:    8192,
			Protocols:         protocols,
			BaseContext: func(net.Listener) context.Context {
				return ctx
			},
			ConnContext: func(ctx context.Context, c net.Conn) context.Context {
				return log.ContextWithNewID(ctx)
			},
		}
	} else {
		server.quicConfig = &quic.Config{
			DisablePathMTUDiscovery: !C.IsLinux && !C.IsWindows,
		}
		server.http3Server = &http3.Server{
			Handler: server,
		}
	}
	return server, nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if len(s.host) > 0 && !isValidHTTPHost(request.Host, s.host) {
		s.logger.ErrorContext(request.Context(), "failed to validate host, request:", request.Host, ", config:", s.host)
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	if !strings.HasPrefix(request.URL.Path, s.path) {
		s.logger.ErrorContext(request.Context(), "failed to validate path, request:", request.URL.Path, ", config:", s.path)
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Methods", "GET, POST")
	writer.Header().Set("X-Padding", strings.Repeat("X", int(s.options.GetNormalizedXPaddingBytes().Rand())))
	validRange := s.options.GetNormalizedXPaddingBytes()
	paddingLength := 0
	referrer := request.Header.Get("Referer")
	if referrer != "" {
		if referrerURL, err := url.Parse(referrer); err == nil {
			// Browser dialer cannot control the host part of referrer header, so only check the query
			paddingLength = len(referrerURL.Query().Get("x_padding"))
		}
	} else {
		paddingLength = len(request.URL.Query().Get("x_padding"))
	}
	if int32(paddingLength) < validRange.From || int32(paddingLength) > validRange.To {
		s.logger.ErrorContext(request.Context(), "invalid x_padding length:", int32(paddingLength))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	sessionId := ""
	subpath := strings.Split(request.URL.Path[len(s.path):], "/")
	if len(subpath) > 0 {
		sessionId = subpath[0]
	}
	if sessionId == "" && s.options.Mode != "" && s.options.Mode != "auto" && s.options.Mode != "stream-one" && s.options.Mode != "stream-up" {
		s.logger.ErrorContext(request.Context(), "stream-one mode is not allowed")
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	forwardedAddrs := parseXForwardedFor(request.Header)
	var remoteAddr net.Addr
	var err error
	remoteAddr, err = net.ResolveTCPAddr("tcp", request.RemoteAddr)
	if err != nil {
		remoteAddr = &net.TCPAddr{
			IP:   []byte{0, 0, 0, 0},
			Port: 0,
		}
	}
	if request.ProtoMajor == 3 {
		remoteAddr = &net.UDPAddr{
			IP:   remoteAddr.(*net.TCPAddr).IP,
			Port: remoteAddr.(*net.TCPAddr).Port,
		}
	}
	if len(forwardedAddrs) > 0 && forwardedAddrs[0].Family().IsIP() {
		remoteAddr = &net.TCPAddr{
			IP:   forwardedAddrs[0].IP(),
			Port: 0,
		}
	}
	var currentSession *httpSession
	if sessionId != "" {
		currentSession = s.upsertSession(sessionId)
	}
	scMaxEachPostBytes := int(s.options.GetNormalizedScMaxEachPostBytes().To)
	if request.Method == "POST" && sessionId != "" { // stream-up, packet-up
		seq := ""
		if len(subpath) > 1 {
			seq = subpath[1]
		}
		if seq == "" {
			if s.options.Mode != "" && s.options.Mode != "auto" && s.options.Mode != "stream-up" {
				s.logger.ErrorContext(request.Context(), "stream-up mode is not allowed")
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			httpSC := &httpServerConn{
				Instance:       done.New(),
				Reader:         request.Body,
				ResponseWriter: writer,
			}
			err = currentSession.uploadQueue.Push(Packet{
				Reader: httpSC,
			})
			if err != nil {
				s.logger.InfoContext(request.Context(), err, "failed to upload (PushReader)")
				writer.WriteHeader(http.StatusConflict)
			} else {
				writer.Header().Set("X-Accel-Buffering", "no")
				writer.Header().Set("Cache-Control", "no-store")
				writer.WriteHeader(http.StatusOK)
				scStreamUpServerSecs := s.options.GetNormalizedScStreamUpServerSecs()
				if referrer != "" && scStreamUpServerSecs.To > 0 {
					go func() {
						for {
							_, err := httpSC.Write(bytes.Repeat([]byte{'X'}, int(s.options.GetNormalizedXPaddingBytes().Rand())))
							if err != nil {
								break
							}
							time.Sleep(time.Duration(scStreamUpServerSecs.Rand()) * time.Second)
						}
					}()
				}
				select {
				case <-request.Context().Done():
				case <-httpSC.Wait():
				}
			}
			httpSC.Close()
			return
		}
		if s.options.Mode != "" && s.options.Mode != "auto" && s.options.Mode != "packet-up" {
			s.logger.ErrorContext(request.Context(), "packet-up mode is not allowed")
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		payload, err := io.ReadAll(io.LimitReader(request.Body, int64(scMaxEachPostBytes)+1))
		if len(payload) > scMaxEachPostBytes {
			s.logger.ErrorContext(request.Context(), "Too large upload. scMaxEachPostBytes is set to ", scMaxEachPostBytes, "but request size exceed it. Adjust scMaxEachPostBytes on the server to be at least as large as client.")
			writer.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		if err != nil {
			s.logger.InfoContext(request.Context(), err, "failed to upload (ReadAll)")
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		seqInt, err := strconv.ParseUint(seq, 10, 64)
		if err != nil {
			s.logger.InfoContext(request.Context(), err, "failed to upload (ParseUint)")
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		err = currentSession.uploadQueue.Push(Packet{
			Payload: payload,
			Seq:     seqInt,
		})
		if err != nil {
			s.logger.InfoContext(request.Context(), err, "failed to upload (PushPayload)")
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		writer.WriteHeader(http.StatusOK)
	} else if request.Method == "GET" || sessionId == "" { // stream-down, stream-one
		if sessionId != "" {
			// after GET is done, the connection is finished. disable automatic
			// session reaping, and handle it in defer
			currentSession.isFullyConnected.Close()
			defer s.sessions.Delete(sessionId)
		}
		// magic header instructs nginx + apache to not buffer response body
		writer.Header().Set("X-Accel-Buffering", "no")
		// A web-compliant header telling all middleboxes to disable caching.
		// Should be able to prevent overloading the cache, or stop CDNs from
		// teeing the response stream into their cache, causing slowdowns.
		writer.Header().Set("Cache-Control", "no-store")
		if !s.options.NoSSEHeader {
			// magic header to make the HTTP middle box consider this as SSE to disable buffer
			writer.Header().Set("Content-Type", "text/event-stream")
		}
		writer.WriteHeader(http.StatusOK)
		writer.(http.Flusher).Flush()
		httpSC := &httpServerConn{
			Instance:       done.New(),
			Reader:         request.Body,
			ResponseWriter: writer,
		}
		conn := splitConn{
			writer:     httpSC,
			reader:     httpSC,
			remoteAddr: remoteAddr,
			localAddr:  s.localAddr,
		}
		if sessionId != "" { // if not stream-one
			conn.reader = currentSession.uploadQueue
		}
		s.handler.NewConnectionEx(request.Context(), &conn, sHttp.SourceAddress(request), M.Socksaddr{}, func(it error) {})
		// "A ResponseWriter may not be used after [Handler.ServeHTTP] has returned."
		select {
		case <-request.Context().Done():
		case <-httpSC.Wait():
		}
		conn.Close()
	} else {
		s.logger.ErrorContext(request.Context(), "unsupported method: ", request.Method)
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) Network() []string {
	return []string{s.network()}
}

func (s *Server) Serve(listener net.Listener) error {
	if s.network() == N.NetworkTCP {
		if s.tlsConfig != nil {
			listener = aTLS.NewListener(listener, s.tlsConfig)
		}
		s.localAddr = listener.Addr()
		return s.httpServer.Serve(listener)
	}
	return os.ErrInvalid
}

func (s *Server) ServePacket(listener net.PacketConn) error {
	if s.network() == N.NetworkUDP {
		quicListener, err := qtls.ListenEarly(listener, s.tlsConfig, s.quicConfig)
		if err != nil {
			return err
		}
		s.localAddr = quicListener.Addr()
		return s.http3Server.ServeListener(quicListener)
	}
	return os.ErrInvalid
}

func (s *Server) Close() error {
	if s.network() == N.NetworkTCP {
		return common.Close(s.httpServer)
	}
	return common.Close(s.http3Server)
}

func (s *Server) network() string {
	if s.tlsConfig != nil && len(s.tlsConfig.NextProtos()) == 1 && s.tlsConfig.NextProtos()[0] == "h3" {
		return N.NetworkUDP
	}
	return N.NetworkTCP
}

func (s *Server) upsertSession(sessionId string) *httpSession {
	// fast path
	currentSessionAny, ok := s.sessions.Load(sessionId)
	if ok {
		return currentSessionAny.(*httpSession)
	}
	// slow path
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	currentSessionAny, ok = s.sessions.Load(sessionId)
	if ok {
		return currentSessionAny.(*httpSession)
	}
	session := &httpSession{
		uploadQueue:      NewUploadQueue(s.options.GetNormalizedScMaxBufferedPosts()),
		isFullyConnected: done.New(),
	}
	s.sessions.Store(sessionId, session)
	shouldReap := done.New()
	go func() {
		time.Sleep(30 * time.Second)
		shouldReap.Close()
	}()
	go func() {
		select {
		case <-shouldReap.Wait():
			s.sessions.Delete(sessionId)
			session.uploadQueue.Close()
		case <-session.isFullyConnected.Wait():
		}
	}()
	return session
}
