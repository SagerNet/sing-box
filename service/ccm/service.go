package ccm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

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

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/go-chi/chi/v5"
	"golang.org/x/net/http2"
)

const (
	contextWindowStandard   = 200000
	contextWindowPremium    = 1000000
	premiumContextThreshold = 200000
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
	accessMutex    sync.RWMutex
	usageTracker   *AggregatedUsage
	trackingGroup  sync.WaitGroup
	shuttingDown   bool
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

	userManager := &UserManager{
		tokenMap: make(map[string]string),
	}

	var usageTracker *AggregatedUsage
	if options.UsagesPath != "" {
		usageTracker = &AggregatedUsage{
			LastUpdated:  time.Now(),
			Combinations: make([]CostCombination, 0),
			filePath:     options.UsagesPath,
			logger:       logger,
		}
	}

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
		userManager:  userManager,
		usageTracker: usageTracker,
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

	if s.usageTracker != nil {
		err = s.usageTracker.Load()
		if err != nil {
			s.logger.Warn("load usage statistics: ", err)
		}
	}

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

	go func() {
		serveErr := s.httpServer.Serve(tcpListener)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			s.logger.Error("serve error: ", serveErr)
		}
	}()

	return nil
}

func (s *Service) getAccessToken() (string, error) {
	s.accessMutex.RLock()
	if !s.credentials.needsRefresh() {
		token := s.credentials.AccessToken
		s.accessMutex.RUnlock()
		return token, nil
	}
	s.accessMutex.RUnlock()

	s.accessMutex.Lock()
	defer s.accessMutex.Unlock()

	if !s.credentials.needsRefresh() {
		return s.credentials.AccessToken, nil
	}

	newCredentials, err := refreshToken(s.httpClient, s.credentials)
	if err != nil {
		return "", err
	}

	s.credentials = newCredentials

	err = platformWriteCredentials(newCredentials, s.credentialPath)
	if err != nil {
		s.logger.Warn("persist refreshed token: ", err)
	}

	return newCredentials.AccessToken, nil
}

func detectContextWindow(betaHeader string, inputTokens int64) int {
	if inputTokens > premiumContextThreshold {
		features := strings.Split(betaHeader, ",")
		for _, feature := range features {
			if strings.TrimSpace(feature) == "context-1m" {
				return contextWindowPremium
			}
		}
	}
	return contextWindowStandard
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/v1/") {
		writeJSONError(w, r, http.StatusNotFound, "not_found_error", "Not found")
		return
	}

	var username string
	if len(s.users) > 0 {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.logger.Warn("authentication failed for request from ", r.RemoteAddr, ": missing Authorization header")
			writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "missing api key")
			return
		}
		clientToken := strings.TrimPrefix(authHeader, "Bearer ")
		if clientToken == authHeader {
			s.logger.Warn("authentication failed for request from ", r.RemoteAddr, ": invalid Authorization format")
			writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "invalid api key format")
			return
		}
		var ok bool
		username, ok = s.userManager.Authenticate(clientToken)
		if !ok {
			s.logger.Warn("authentication failed for request from ", r.RemoteAddr, ": unknown key: ", clientToken)
			writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "invalid api key")
			return
		}
	}

	var requestModel string
	var messagesCount int

	if s.usageTracker != nil && r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			var request struct {
				Model    string                   `json:"model"`
				Messages []anthropic.MessageParam `json:"messages"`
			}
			err := json.Unmarshal(bodyBytes, &request)
			if err == nil {
				requestModel = request.Model
				messagesCount = len(request.Messages)
			}
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
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
		if !isHopByHopHeader(key) && key != "Authorization" {
			proxyRequest.Header[key] = values
		}
	}

	anthropicBetaHeader := proxyRequest.Header.Get("anthropic-beta")
	if anthropicBetaHeader != "" {
		proxyRequest.Header.Set("anthropic-beta", anthropicBetaOAuthValue+","+anthropicBetaHeader)
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
		writeJSONError(w, r, http.StatusBadGateway, "api_error", err.Error())
		return
	}
	defer response.Body.Close()

	for key, values := range response.Header {
		if !isHopByHopHeader(key) {
			w.Header()[key] = values
		}
	}
	w.WriteHeader(response.StatusCode)

	if s.usageTracker != nil && response.StatusCode == http.StatusOK {
		s.handleResponseWithTracking(w, response, requestModel, anthropicBetaHeader, messagesCount, username)
	} else {
		mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
		if err == nil && mediaType != "text/event-stream" {
			_, _ = io.Copy(w, response.Body)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			s.logger.Error("streaming not supported")
			return
		}
		buffer := make([]byte, buf.BufferSize)
		for {
			n, err := response.Body.Read(buffer)
			if n > 0 {
				_, writeError := w.Write(buffer[:n])
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
}

func (s *Service) handleResponseWithTracking(writer http.ResponseWriter, response *http.Response, requestModel string, anthropicBetaHeader string, messagesCount int, username string) {
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	isStreaming := err == nil && mediaType == "text/event-stream"

	if !isStreaming {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			s.logger.Error("read response body: ", err)
			return
		}

		var message anthropic.Message
		var usage anthropic.Usage
		var responseModel string
		err = json.Unmarshal(bodyBytes, &message)
		if err == nil {
			responseModel = string(message.Model)
			usage = message.Usage
		}
		if responseModel == "" {
			responseModel = requestModel
		}

		if usage.InputTokens > 0 || usage.OutputTokens > 0 {
			if responseModel != "" {
				contextWindow := detectContextWindow(anthropicBetaHeader, usage.InputTokens)
				s.usageTracker.AddUsage(
					responseModel,
					contextWindow,
					messagesCount,
					usage.InputTokens,
					usage.OutputTokens,
					usage.CacheReadInputTokens,
					usage.CacheCreationInputTokens,
					username,
				)
			}
		}

		_, _ = writer.Write(bodyBytes)
		return
	}

	flusher, ok := writer.(http.Flusher)
	if !ok {
		s.logger.Error("streaming not supported")
		return
	}

	var accumulatedUsage anthropic.Usage
	var responseModel string
	buffer := make([]byte, buf.BufferSize)
	var leftover []byte

	for {
		n, err := response.Body.Read(buffer)
		if n > 0 {
			data := append(leftover, buffer[:n]...)
			lines := bytes.Split(data, []byte("\n"))

			if err == nil {
				leftover = lines[len(lines)-1]
				lines = lines[:len(lines)-1]
			} else {
				leftover = nil
			}

			for _, line := range lines {
				line = bytes.TrimSpace(line)
				if len(line) == 0 {
					continue
				}

				if bytes.HasPrefix(line, []byte("data: ")) {
					eventData := bytes.TrimPrefix(line, []byte("data: "))
					if bytes.Equal(eventData, []byte("[DONE]")) {
						continue
					}

					var event anthropic.MessageStreamEventUnion
					err := json.Unmarshal(eventData, &event)
					if err != nil {
						continue
					}
					switch event.Type {
					case "message_start":
						messageStart := event.AsMessageStart()
						if messageStart.Message.Model != "" {
							responseModel = string(messageStart.Message.Model)
						}
						if messageStart.Message.Usage.InputTokens > 0 {
							accumulatedUsage.InputTokens = messageStart.Message.Usage.InputTokens
							accumulatedUsage.CacheReadInputTokens = messageStart.Message.Usage.CacheReadInputTokens
							accumulatedUsage.CacheCreationInputTokens = messageStart.Message.Usage.CacheCreationInputTokens
						}
					case "message_delta":
						messageDelta := event.AsMessageDelta()
						if messageDelta.Usage.OutputTokens > 0 {
							accumulatedUsage.OutputTokens = messageDelta.Usage.OutputTokens
						}
					}
				}
			}

			_, writeError := writer.Write(buffer[:n])
			if writeError != nil {
				s.logger.Error("write streaming response: ", writeError)
				return
			}
			flusher.Flush()
		}

		if err != nil {
			if responseModel == "" {
				responseModel = requestModel
			}

			if accumulatedUsage.InputTokens > 0 || accumulatedUsage.OutputTokens > 0 {
				if responseModel != "" {
					contextWindow := detectContextWindow(anthropicBetaHeader, accumulatedUsage.InputTokens)
					s.usageTracker.AddUsage(
						responseModel,
						contextWindow,
						messagesCount,
						accumulatedUsage.InputTokens,
						accumulatedUsage.OutputTokens,
						accumulatedUsage.CacheReadInputTokens,
						accumulatedUsage.CacheCreationInputTokens,
						username,
					)
				}
			}
			return
		}
	}
}

func (s *Service) Close() error {
	err := common.Close(
		common.PtrOrNil(s.httpServer),
		common.PtrOrNil(s.listener),
		s.tlsConfig,
	)

	if s.usageTracker != nil {
		s.usageTracker.cancelPendingSave()
		saveErr := s.usageTracker.Save()
		if saveErr != nil {
			s.logger.Error("save usage statistics: ", saveErr)
		}
	}

	return err
}
