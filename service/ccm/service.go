package ccm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/http2"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.CCMServiceOptions](registry, C.TypeCCM, NewService)
}

type errorResponse struct {
	Type      string       `json:"type"`
	Error     errorDetails `json:"error"`
	RequestID string       `json:"request_id,omitempty"`
}

type errorDetails struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func writeJSONError(w http.ResponseWriter, r *http.Request, statusCode int, errorType string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	json.NewEncoder(w).Encode(errorResponse{
		Type: "error",
		Error: errorDetails{
			Type:    errorType,
			Message: message,
		},
		RequestID: r.Header.Get("Request-Id"),
	})
}

func isHopByHopHeader(header string) bool {
	switch strings.ToLower(header) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailers", "transfer-encoding", "upgrade", "host":
		return true
	default:
		return false
	}
}

type Service struct {
	boxService.Adapter
	ctx            context.Context
	logger         log.ContextLogger
	credentialPath string
	credentials    *oauthCredentials
	users          []option.CCMUser
	httpClient     *http.Client
	httpHeaders    http.Header
	listener       *listener.Listener
	tlsConfig      tls.ServerConfig
	httpServer     *http.Server
	userManager    *UserManager
	access         sync.RWMutex
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.CCMServiceOptions) (adapter.Service, error) {
	serviceDialer, err := dialer.NewWithOptions(dialer.Options{
		Context: ctx,
		Options: option.DialerOptions{
			Detour: options.Detour,
		},
		RemoteIsDomain: true,
	})
	if err != nil {
		return nil, E.Cause(err, "create dialer")
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return serviceDialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}

	userManager := NewUserManager()

	service := &Service{
		Adapter:        boxService.NewAdapter(C.TypeCCM, tag),
		ctx:            ctx,
		logger:         logger,
		credentialPath: options.CredentialPath,
		users:          options.Users,
		httpClient:     httpClient,
		httpHeaders:    options.Headers.Build(),
		listener: listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Network: []string{N.NetworkTCP},
			Listen:  options.ListenOptions,
		}),
		userManager: userManager,
	}

	if options.TLS != nil {
		tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
		service.tlsConfig = tlsConfig
	}

	return service, nil
}

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}

	s.userManager.UpdateUsers(s.users)

	credentials, err := platformReadCredentials(s.credentialPath)
	if err != nil {
		return E.Cause(err, "read credentials")
	}
	s.credentials = credentials

	router := chi.NewRouter()
	router.Mount("/", s)

	s.httpServer = &http.Server{Handler: router}

	if s.tlsConfig != nil {
		err = s.tlsConfig.Start()
		if err != nil {
			return E.Cause(err, "create TLS config")
		}
	}

	tcpListener, err := s.listener.ListenTCP()
	if err != nil {
		return err
	}

	if s.tlsConfig != nil {
		if !common.Contains(s.tlsConfig.NextProtos(), http2.NextProtoTLS) {
			s.tlsConfig.SetNextProtos(append([]string{"h2"}, s.tlsConfig.NextProtos()...))
		}
		tcpListener = aTLS.NewListener(tcpListener, s.tlsConfig)
	}

	s.logger.Info("ccm service listening on ", tcpListener.Addr())

	go func() {
		serveErr := s.httpServer.Serve(tcpListener)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			s.logger.Error("serve error: ", serveErr)
		}
	}()

	return nil
}

func (s *Service) getAccessToken() (string, error) {
	s.access.RLock()
	if !s.credentials.needsRefresh() {
		token := s.credentials.AccessToken
		s.access.RUnlock()
		return token, nil
	}
	s.access.RUnlock()

	s.access.Lock()
	defer s.access.Unlock()

	if !s.credentials.needsRefresh() {
		return s.credentials.AccessToken, nil
	}

	s.logger.Info("refreshing OAuth token")
	newCredentials, err := refreshToken(s.httpClient, s.credentials)
	if err != nil {
		return "", err
	}

	s.credentials = newCredentials

	err = platformWriteCredentials(newCredentials, s.credentialPath)
	if err != nil {
		s.logger.Warn("persist refreshed token: ", err)
	} else {
		s.logger.Info("OAuth token refreshed successfully")
	}

	return newCredentials.AccessToken, nil
}

func (s *Service) authenticateRequest(r *http.Request) bool {
	if len(s.users) == 0 {
		return true
	}
	clientToken := r.Header.Get("x-api-key")
	if clientToken == "" {
		return false
	}
	_, ok := s.userManager.Authenticate(clientToken)
	return ok
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/v1/") {
		writeJSONError(w, r, http.StatusNotFound, "not_found_error", "Not found")
		return
	}

	if !s.authenticateRequest(r) {
		s.logger.Warn("authentication failed for request from ", r.RemoteAddr)
		writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "invalid x-api-key")
		return
	}

	accessToken, err := s.getAccessToken()
	if err != nil {
		s.logger.Error("get access token: ", err)
		writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "Authentication failed")
		return
	}

	proxyURL := claudeAPIBaseURL + r.URL.RequestURI()
	proxyRequest, err := http.NewRequestWithContext(r.Context(), r.Method, proxyURL, r.Body)
	if err != nil {
		s.logger.Error("create proxy request: ", err)
		writeJSONError(w, r, http.StatusInternalServerError, "api_error", "Internal server error")
		return
	}

	for key, values := range r.Header {
		if !isHopByHopHeader(key) && key != "x-api-key" {
			proxyRequest.Header[key] = values
		}
	}

	if betaHeader := proxyRequest.Header.Get("anthropic-beta"); betaHeader != "" {
		proxyRequest.Header.Set("anthropic-beta", anthropicBetaOAuthValue+","+betaHeader)
	} else {
		proxyRequest.Header.Set("anthropic-beta", anthropicBetaOAuthValue)
	}

	for key, values := range s.httpHeaders {
		proxyRequest.Header.Del(key)
		proxyRequest.Header[key] = values
	}

	proxyRequest.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := s.httpClient.Do(proxyRequest)
	if err != nil {
		s.logger.Error("send request to Claude API: ", err)
		writeJSONError(w, r, http.StatusBadGateway, "api_error", "Failed to connect to Claude API")
		return
	}
	defer response.Body.Close()

	for key, values := range response.Header {
		if !isHopByHopHeader(key) {
			w.Header()[key] = values
		}
	}
	w.WriteHeader(response.StatusCode)
	s.handleResponse(w, response)
}

func (s *Service) handleResponse(writer http.ResponseWriter, response *http.Response) {
	if mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type")); err == nil && mediaType != "text/event-stream" {
		_, _ = io.Copy(writer, response.Body)
		return
	}
	flusher, ok := writer.(http.Flusher)
	if !ok {
		s.logger.Error("streaming not supported")
		return
	}
	buffer := make([]byte, buf.BufferSize)
	for {
		n, err := response.Body.Read(buffer)
		if n > 0 {
			_, writeError := writer.Write(buffer[:n])
			if writeError != nil {
				s.logger.Error("write streaming response: ", writeError)
				return
			}
			flusher.Flush()
		}
		if err != nil {
			return
		}
	}
}

func (s *Service) Close() error {
	return common.Close(
		common.PtrOrNil(s.httpServer),
		common.PtrOrNil(s.listener),
		s.tlsConfig,
	)
}
