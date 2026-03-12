package ocm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"

	"github.com/go-chi/chi/v5"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"golang.org/x/net/http2"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.OCMServiceOptions](registry, C.TypeOCM, NewService)
}

type errorResponse struct {
	Error errorDetails `json:"error"`
}

type errorDetails struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

func writeJSONError(w http.ResponseWriter, r *http.Request, statusCode int, errorType string, message string) {
	writeJSONErrorWithCode(w, r, statusCode, errorType, "", message)
}

func writeJSONErrorWithCode(w http.ResponseWriter, r *http.Request, statusCode int, errorType string, errorCode string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	json.NewEncoder(w).Encode(errorResponse{
		Error: errorDetails{
			Type:    errorType,
			Code:    errorCode,
			Message: message,
		},
	})
}

func writePlainTextError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(statusCode)
	_, _ = io.WriteString(w, message)
}

const (
	retryableUsageMessage = "current credential reached its usage limit; retry the request to use another credential"
	retryableUsageCode    = "credential_usage_exhausted"
)

func hasAlternativeCredential(provider credentialProvider, currentCredential credential, filter func(credential) bool) bool {
	if provider == nil || currentCredential == nil {
		return false
	}
	for _, cred := range provider.allCredentials() {
		if cred == currentCredential {
			continue
		}
		if filter != nil && !filter(cred) {
			continue
		}
		if cred.isUsable() {
			return true
		}
	}
	return false
}

func unavailableCredentialMessage(provider credentialProvider, fallback string) string {
	if provider == nil {
		return fallback
	}
	return allRateLimitedError(provider.allCredentials()).Error()
}

func writeRetryableUsageError(w http.ResponseWriter, r *http.Request) {
	writeJSONErrorWithCode(w, r, http.StatusServiceUnavailable, "server_error", retryableUsageCode, retryableUsageMessage)
}

func writeNonRetryableCredentialError(w http.ResponseWriter, message string) {
	writePlainTextError(w, http.StatusBadRequest, message)
}

func writeCredentialUnavailableError(
	w http.ResponseWriter,
	r *http.Request,
	provider credentialProvider,
	currentCredential credential,
	filter func(credential) bool,
	fallback string,
) {
	if hasAlternativeCredential(provider, currentCredential, filter) {
		writeRetryableUsageError(w, r)
		return
	}
	writeNonRetryableCredentialError(w, unavailableCredentialMessage(provider, fallback))
}

func isHopByHopHeader(header string) bool {
	switch strings.ToLower(header) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailers", "transfer-encoding", "upgrade", "host":
		return true
	default:
		return false
	}
}

func normalizeRateLimitIdentifier(limitIdentifier string) string {
	trimmedIdentifier := strings.TrimSpace(strings.ToLower(limitIdentifier))
	if trimmedIdentifier == "" {
		return ""
	}
	return strings.ReplaceAll(trimmedIdentifier, "_", "-")
}

func parseInt64Header(headers http.Header, headerName string) (int64, bool) {
	headerValue := strings.TrimSpace(headers.Get(headerName))
	if headerValue == "" {
		return 0, false
	}
	parsedValue, parseError := strconv.ParseInt(headerValue, 10, 64)
	if parseError != nil {
		return 0, false
	}
	return parsedValue, true
}

func weeklyCycleHintForLimit(headers http.Header, limitIdentifier string) *WeeklyCycleHint {
	normalizedLimitIdentifier := normalizeRateLimitIdentifier(limitIdentifier)
	if normalizedLimitIdentifier == "" {
		return nil
	}

	windowHeader := "x-" + normalizedLimitIdentifier + "-secondary-window-minutes"
	resetHeader := "x-" + normalizedLimitIdentifier + "-secondary-reset-at"

	windowMinutes, hasWindowMinutes := parseInt64Header(headers, windowHeader)
	resetAtUnix, hasResetAt := parseInt64Header(headers, resetHeader)
	if !hasWindowMinutes || !hasResetAt || windowMinutes <= 0 || resetAtUnix <= 0 {
		return nil
	}

	return &WeeklyCycleHint{
		WindowMinutes: windowMinutes,
		ResetAt:       time.Unix(resetAtUnix, 0).UTC(),
	}
}

func extractWeeklyCycleHint(headers http.Header) *WeeklyCycleHint {
	activeLimitIdentifier := normalizeRateLimitIdentifier(headers.Get("x-codex-active-limit"))
	if activeLimitIdentifier != "" {
		if activeHint := weeklyCycleHintForLimit(headers, activeLimitIdentifier); activeHint != nil {
			return activeHint
		}
	}
	return weeklyCycleHintForLimit(headers, "codex")
}

type Service struct {
	boxService.Adapter
	ctx            context.Context
	logger         log.ContextLogger
	options        option.OCMServiceOptions
	httpHeaders    http.Header
	listener       *listener.Listener
	tlsConfig      tls.ServerConfig
	httpServer     *http.Server
	userManager    *UserManager
	webSocketMutex sync.Mutex
	webSocketGroup sync.WaitGroup
	webSocketConns map[*webSocketSession]struct{}
	shuttingDown   bool

	// Legacy mode
	legacyCredential *defaultCredential
	legacyProvider   credentialProvider

	// Multi-credential mode
	providers      map[string]credentialProvider
	allCredentials []credential
	userConfigMap  map[string]*option.OCMUser
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.OCMServiceOptions) (adapter.Service, error) {
	err := validateOCMOptions(options)
	if err != nil {
		return nil, E.Cause(err, "validate options")
	}

	userManager := &UserManager{
		tokenMap: make(map[string]string),
	}

	service := &Service{
		Adapter:     boxService.NewAdapter(C.TypeOCM, tag),
		ctx:         ctx,
		logger:      logger,
		options:     options,
		httpHeaders: options.Headers.Build(),
		listener: listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Network: []string{N.NetworkTCP},
			Listen:  options.ListenOptions,
		}),
		userManager:    userManager,
		webSocketConns: make(map[*webSocketSession]struct{}),
	}

	if len(options.Credentials) > 0 {
		providers, allCredentials, err := buildOCMCredentialProviders(ctx, options, logger)
		if err != nil {
			return nil, E.Cause(err, "build credential providers")
		}
		service.providers = providers
		service.allCredentials = allCredentials

		userConfigMap := make(map[string]*option.OCMUser)
		for i := range options.Users {
			userConfigMap[options.Users[i].Name] = &options.Users[i]
		}
		service.userConfigMap = userConfigMap
	} else {
		cred, err := newDefaultCredential(ctx, "default", option.OCMDefaultCredentialOptions{
			CredentialPath: options.CredentialPath,
			UsagesPath:     options.UsagesPath,
			Detour:         options.Detour,
		}, logger)
		if err != nil {
			return nil, err
		}
		service.legacyCredential = cred
		service.legacyProvider = &singleCredentialProvider{cred: cred}
		service.allCredentials = []credential{cred}
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

	s.userManager.UpdateUsers(s.options.Users)

	for _, cred := range s.allCredentials {
		err := cred.start()
		if err != nil {
			return err
		}
		tag := cred.tagName()
		cred.setOnBecameUnusable(func() {
			s.interruptWebSocketSessionsForCredential(tag)
		})
	}
	if len(s.options.Credentials) > 0 {
		err := validateOCMCompositeCredentialModes(s.options, s.providers)
		if err != nil {
			return E.Cause(err, "validate loaded credentials")
		}
	}

	router := chi.NewRouter()
	router.Mount("/", s)

	s.httpServer = &http.Server{Handler: router}

	if s.tlsConfig != nil {
		err := s.tlsConfig.Start()
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

func (s *Service) resolveCredentialProvider(username string) (credentialProvider, error) {
	if len(s.options.Users) > 0 {
		return credentialForUser(s.userConfigMap, s.providers, s.legacyProvider, username)
	}
	provider := noUserCredentialProvider(s.providers, s.legacyProvider, s.options)
	if provider == nil {
		return nil, E.New("no credential available")
	}
	return provider, nil
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/ocm/v1/status" {
		s.handleStatusEndpoint(w, r)
		return
	}

	path := r.URL.Path
	if !strings.HasPrefix(path, "/v1/") {
		writeJSONError(w, r, http.StatusNotFound, "invalid_request_error", "path must start with /v1/")
		return
	}

	var username string
	if len(s.options.Users) > 0 {
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

	sessionID := r.Header.Get("session_id")

	// Resolve credential provider and user config
	var provider credentialProvider
	var userConfig *option.OCMUser
	if len(s.options.Users) > 0 {
		userConfig = s.userConfigMap[username]
		var err error
		provider, err = credentialForUser(s.userConfigMap, s.providers, s.legacyProvider, username)
		if err != nil {
			s.logger.Error("resolve credential: ", err)
			writeJSONError(w, r, http.StatusInternalServerError, "api_error", err.Error())
			return
		}
	} else {
		provider = noUserCredentialProvider(s.providers, s.legacyProvider, s.options)
	}
	if provider == nil {
		writeJSONError(w, r, http.StatusInternalServerError, "api_error", "no credential available")
		return
	}

	provider.pollIfStale(s.ctx)

	var credentialFilter func(credential) bool
	if userConfig != nil && !userConfig.AllowExternalUsage {
		credentialFilter = func(c credential) bool { return !c.isExternal() }
	}

	selectedCredential, isNew, err := provider.selectCredential(sessionID, credentialFilter)
	if err != nil {
		writeNonRetryableCredentialError(w, unavailableCredentialMessage(provider, err.Error()))
		return
	}
	if isNew {
		if username != "" {
			s.logger.Debug("assigned credential ", selectedCredential.tagName(), " for session ", sessionID, " by user ", username)
		} else {
			s.logger.Debug("assigned credential ", selectedCredential.tagName(), " for session ", sessionID)
		}
	}

	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") && strings.HasPrefix(path, "/v1/responses") {
		s.handleWebSocket(w, r, path, username, sessionID, userConfig, provider, selectedCredential, credentialFilter)
		return
	}

	if !selectedCredential.isExternal() && selectedCredential.ocmIsAPIKeyMode() {
		// API key mode path handling
	} else if !selectedCredential.isExternal() {
		if path == "/v1/chat/completions" {
			writeJSONError(w, r, http.StatusBadRequest, "invalid_request_error",
				"chat completions endpoint is only available in API key mode")
			return
		}
	}

	shouldTrackUsage := selectedCredential.usageTrackerOrNil() != nil &&
		(path == "/v1/chat/completions" || strings.HasPrefix(path, "/v1/responses"))
	canRetryRequest := len(provider.allCredentials()) > 1

	// Read body for model extraction and retry buffer when JSON replay is useful.
	var bodyBytes []byte
	var requestModel string
	if r.Body != nil && (shouldTrackUsage || canRetryRequest) {
		mediaType, _, parseErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
		isJSONRequest := parseErr == nil && (mediaType == "application/json" || strings.HasSuffix(mediaType, "+json"))
		if isJSONRequest {
			bodyBytes, err = io.ReadAll(r.Body)
			if err != nil {
				s.logger.Error("read request body: ", err)
				writeJSONError(w, r, http.StatusInternalServerError, "api_error", "failed to read request body")
				return
			}
			var request struct {
				Model string `json:"model"`
			}
			if json.Unmarshal(bodyBytes, &request) == nil {
				requestModel = request.Model
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	}

	requestContext := selectedCredential.wrapRequestContext(r.Context())
	defer func() {
		requestContext.cancelRequest()
	}()
	proxyRequest, err := selectedCredential.buildProxyRequest(requestContext, r, bodyBytes, s.httpHeaders)
	if err != nil {
		s.logger.Error("create proxy request: ", err)
		writeJSONError(w, r, http.StatusInternalServerError, "api_error", "Internal server error")
		return
	}

	response, err := selectedCredential.httpTransport().Do(proxyRequest)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		if requestContext.Err() != nil {
			writeCredentialUnavailableError(w, r, provider, selectedCredential, credentialFilter, "credential became unavailable while processing the request")
			return
		}
		writeJSONError(w, r, http.StatusBadGateway, "api_error", err.Error())
		return
	}
	requestContext.releaseCredentialInterrupt()

	// Transparent 429 retry
	for response.StatusCode == http.StatusTooManyRequests {
		resetAt := parseOCMRateLimitResetFromHeaders(response.Header)
		nextCredential := provider.onRateLimited(sessionID, selectedCredential, resetAt, credentialFilter)
		needsBodyReplay := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodDelete
		selectedCredential.updateStateFromHeaders(response.Header)
		if (needsBodyReplay && bodyBytes == nil) || nextCredential == nil {
			response.Body.Close()
			writeCredentialUnavailableError(w, r, provider, selectedCredential, credentialFilter, "all credentials rate-limited")
			return
		}
		response.Body.Close()
		s.logger.Info("retrying with credential ", nextCredential.tagName(), " after 429 from ", selectedCredential.tagName())
		requestContext.cancelRequest()
		requestContext = nextCredential.wrapRequestContext(r.Context())
		retryRequest, buildErr := nextCredential.buildProxyRequest(requestContext, r, bodyBytes, s.httpHeaders)
		if buildErr != nil {
			s.logger.Error("retry request: ", buildErr)
			writeJSONError(w, r, http.StatusBadGateway, "api_error", buildErr.Error())
			return
		}
		retryResponse, retryErr := nextCredential.httpTransport().Do(retryRequest)
		if retryErr != nil {
			if r.Context().Err() != nil {
				return
			}
			if requestContext.Err() != nil {
				writeCredentialUnavailableError(w, r, provider, nextCredential, credentialFilter, "credential became unavailable while retrying the request")
				return
			}
			s.logger.Error("retry request: ", retryErr)
			writeJSONError(w, r, http.StatusBadGateway, "api_error", retryErr.Error())
			return
		}
		requestContext.releaseCredentialInterrupt()
		response = retryResponse
		selectedCredential = nextCredential
	}
	defer response.Body.Close()

	selectedCredential.updateStateFromHeaders(response.Header)

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusTooManyRequests {
		body, _ := io.ReadAll(response.Body)
		s.logger.Error("upstream error from ", selectedCredential.tagName(), ": status ", response.StatusCode, " ", string(body))
		writeJSONError(w, r, http.StatusInternalServerError, "api_error",
			"proxy request (status "+strconv.Itoa(response.StatusCode)+"): "+string(body))
		return
	}

	// Rewrite response headers for external users
	if userConfig != nil && userConfig.ExternalCredential != "" {
		s.rewriteResponseHeadersForExternalUser(response.Header, userConfig)
	}

	for key, values := range response.Header {
		if !isHopByHopHeader(key) {
			w.Header()[key] = values
		}
	}
	w.WriteHeader(response.StatusCode)

	usageTracker := selectedCredential.usageTrackerOrNil()
	if usageTracker != nil && response.StatusCode == http.StatusOK &&
		(path == "/v1/chat/completions" || strings.HasPrefix(path, "/v1/responses")) {
		s.handleResponseWithTracking(w, response, usageTracker, path, requestModel, username)
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

func (s *Service) handleResponseWithTracking(writer http.ResponseWriter, response *http.Response, usageTracker *AggregatedUsage, path string, requestModel string, username string) {
	isChatCompletions := path == "/v1/chat/completions"
	weeklyCycleHint := extractWeeklyCycleHint(response.Header)
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	isStreaming := err == nil && mediaType == "text/event-stream"
	if !isStreaming && !isChatCompletions && response.Header.Get("Content-Type") == "" {
		isStreaming = true
	}
	if !isStreaming {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			s.logger.Error("read response body: ", err)
			return
		}

		var responseModel, serviceTier string
		var inputTokens, outputTokens, cachedTokens int64

		if isChatCompletions {
			var chatCompletion openai.ChatCompletion
			if json.Unmarshal(bodyBytes, &chatCompletion) == nil {
				responseModel = chatCompletion.Model
				serviceTier = string(chatCompletion.ServiceTier)
				inputTokens = chatCompletion.Usage.PromptTokens
				outputTokens = chatCompletion.Usage.CompletionTokens
				cachedTokens = chatCompletion.Usage.PromptTokensDetails.CachedTokens
			}
		} else {
			var responsesResponse responses.Response
			if json.Unmarshal(bodyBytes, &responsesResponse) == nil {
				responseModel = string(responsesResponse.Model)
				serviceTier = string(responsesResponse.ServiceTier)
				inputTokens = responsesResponse.Usage.InputTokens
				outputTokens = responsesResponse.Usage.OutputTokens
				cachedTokens = responsesResponse.Usage.InputTokensDetails.CachedTokens
			}
		}

		if inputTokens > 0 || outputTokens > 0 {
			if responseModel == "" {
				responseModel = requestModel
			}
			if responseModel != "" {
				contextWindow := detectContextWindow(responseModel, serviceTier, inputTokens)
				usageTracker.AddUsageWithCycleHint(
					responseModel,
					contextWindow,
					inputTokens,
					outputTokens,
					cachedTokens,
					serviceTier,
					username,
					time.Now(),
					weeklyCycleHint,
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

	var inputTokens, outputTokens, cachedTokens int64
	var responseModel, serviceTier string
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

					if isChatCompletions {
						var chatChunk openai.ChatCompletionChunk
						if json.Unmarshal(eventData, &chatChunk) == nil {
							if chatChunk.Model != "" {
								responseModel = chatChunk.Model
							}
							if chatChunk.ServiceTier != "" {
								serviceTier = string(chatChunk.ServiceTier)
							}
							if chatChunk.Usage.PromptTokens > 0 {
								inputTokens = chatChunk.Usage.PromptTokens
								cachedTokens = chatChunk.Usage.PromptTokensDetails.CachedTokens
							}
							if chatChunk.Usage.CompletionTokens > 0 {
								outputTokens = chatChunk.Usage.CompletionTokens
							}
						}
					} else {
						var streamEvent responses.ResponseStreamEventUnion
						if json.Unmarshal(eventData, &streamEvent) == nil {
							if streamEvent.Type == "response.completed" {
								completedEvent := streamEvent.AsResponseCompleted()
								if string(completedEvent.Response.Model) != "" {
									responseModel = string(completedEvent.Response.Model)
								}
								if completedEvent.Response.ServiceTier != "" {
									serviceTier = string(completedEvent.Response.ServiceTier)
								}
								if completedEvent.Response.Usage.InputTokens > 0 {
									inputTokens = completedEvent.Response.Usage.InputTokens
									cachedTokens = completedEvent.Response.Usage.InputTokensDetails.CachedTokens
								}
								if completedEvent.Response.Usage.OutputTokens > 0 {
									outputTokens = completedEvent.Response.Usage.OutputTokens
								}
							}
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

			if inputTokens > 0 || outputTokens > 0 {
				if responseModel != "" {
					contextWindow := detectContextWindow(responseModel, serviceTier, inputTokens)
					usageTracker.AddUsageWithCycleHint(
						responseModel,
						contextWindow,
						inputTokens,
						outputTokens,
						cachedTokens,
						serviceTier,
						username,
						time.Now(),
						weeklyCycleHint,
					)
				}
			}
			return
		}
	}
}

func (s *Service) handleStatusEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, r, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed")
		return
	}

	if len(s.options.Users) == 0 {
		writeJSONError(w, r, http.StatusForbidden, "authentication_error", "status endpoint requires user authentication")
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "missing api key")
		return
	}
	clientToken := strings.TrimPrefix(authHeader, "Bearer ")
	if clientToken == authHeader {
		writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "invalid api key format")
		return
	}
	username, ok := s.userManager.Authenticate(clientToken)
	if !ok {
		writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "invalid api key")
		return
	}

	userConfig := s.userConfigMap[username]
	if userConfig == nil {
		writeJSONError(w, r, http.StatusInternalServerError, "api_error", "user config not found")
		return
	}

	provider, err := credentialForUser(s.userConfigMap, s.providers, s.legacyProvider, username)
	if err != nil {
		writeJSONError(w, r, http.StatusInternalServerError, "api_error", err.Error())
		return
	}

	provider.pollIfStale(r.Context())
	avgFiveHour, avgWeekly := s.computeAggregatedUtilization(provider, userConfig)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]float64{
		"five_hour_utilization": avgFiveHour,
		"weekly_utilization":    avgWeekly,
	})
}

func (s *Service) computeAggregatedUtilization(provider credentialProvider, userConfig *option.OCMUser) (float64, float64) {
	var totalFiveHour, totalWeekly float64
	var count int
	for _, cred := range provider.allCredentials() {
		if userConfig.ExternalCredential != "" && cred.tagName() == userConfig.ExternalCredential {
			continue
		}
		if !userConfig.AllowExternalUsage && cred.isExternal() {
			continue
		}
		totalFiveHour += cred.fiveHourUtilization()
		totalWeekly += cred.weeklyUtilization()
		count++
	}
	if count == 0 {
		return 100, 100
	}
	return totalFiveHour / float64(count), totalWeekly / float64(count)
}

func (s *Service) rewriteResponseHeadersForExternalUser(headers http.Header, userConfig *option.OCMUser) {
	provider, err := credentialForUser(s.userConfigMap, s.providers, s.legacyProvider, userConfig.Name)
	if err != nil {
		return
	}

	avgFiveHour, avgWeekly := s.computeAggregatedUtilization(provider, userConfig)

	activeLimitIdentifier := normalizeRateLimitIdentifier(headers.Get("x-codex-active-limit"))
	if activeLimitIdentifier == "" {
		activeLimitIdentifier = "codex"
	}

	headers.Set("x-"+activeLimitIdentifier+"-primary-used-percent", strconv.FormatFloat(avgFiveHour, 'f', 2, 64))
	headers.Set("x-"+activeLimitIdentifier+"-secondary-used-percent", strconv.FormatFloat(avgWeekly, 'f', 2, 64))
}

func (s *Service) Close() error {
	webSocketSessions := s.startWebSocketShutdown()

	err := common.Close(
		common.PtrOrNil(s.httpServer),
		common.PtrOrNil(s.listener),
		s.tlsConfig,
	)
	for _, session := range webSocketSessions {
		session.Close()
	}
	s.webSocketGroup.Wait()

	for _, cred := range s.allCredentials {
		cred.close()
	}

	return err
}

func (s *Service) registerWebSocketSession(session *webSocketSession) bool {
	s.webSocketMutex.Lock()
	defer s.webSocketMutex.Unlock()

	if s.shuttingDown {
		return false
	}

	s.webSocketConns[session] = struct{}{}
	s.webSocketGroup.Add(1)
	return true
}

func (s *Service) unregisterWebSocketSession(session *webSocketSession) {
	s.webSocketMutex.Lock()
	_, loaded := s.webSocketConns[session]
	if loaded {
		delete(s.webSocketConns, session)
	}
	s.webSocketMutex.Unlock()

	if loaded {
		s.webSocketGroup.Done()
	}
}

func (s *Service) isShuttingDown() bool {
	s.webSocketMutex.Lock()
	defer s.webSocketMutex.Unlock()
	return s.shuttingDown
}

func (s *Service) interruptWebSocketSessionsForCredential(tag string) {
	s.webSocketMutex.Lock()
	var toClose []*webSocketSession
	for session := range s.webSocketConns {
		if session.credentialTag == tag {
			toClose = append(toClose, session)
		}
	}
	s.webSocketMutex.Unlock()
	for _, session := range toClose {
		session.Close()
	}
}

func (s *Service) startWebSocketShutdown() []*webSocketSession {
	s.webSocketMutex.Lock()
	defer s.webSocketMutex.Unlock()

	s.shuttingDown = true

	webSocketSessions := make([]*webSocketSession, 0, len(s.webSocketConns))
	for session := range s.webSocketConns {
		webSocketSessions = append(webSocketSessions, session)
	}
	return webSocketSessions
}
