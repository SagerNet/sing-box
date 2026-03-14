package ccm

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

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/go-chi/chi/v5"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	contextWindowStandard   = 200000
	contextWindowPremium    = 1000000
	premiumContextThreshold = 200000
	retryableUsageMessage   = "current credential reached its usage limit; retry the request to use another credential"
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
	message := allCredentialsUnavailableError(provider.allCredentials()).Error()
	if message == "all credentials unavailable" && fallback != "" {
		return fallback
	}
	return message
}

func writeRetryableUsageError(w http.ResponseWriter, r *http.Request) {
	writeJSONError(w, r, http.StatusTooManyRequests, "rate_limit_error", retryableUsageMessage)
}

func writeNonRetryableCredentialError(w http.ResponseWriter, r *http.Request, message string) {
	writeJSONError(w, r, http.StatusBadRequest, "invalid_request_error", message)
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
	writeNonRetryableCredentialError(w, r, unavailableCredentialMessage(provider, fallback))
}

func isHopByHopHeader(header string) bool {
	switch strings.ToLower(header) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailers", "transfer-encoding", "upgrade", "host":
		return true
	default:
		return false
	}
}

func isReverseProxyHeader(header string) bool {
	lowerHeader := strings.ToLower(header)
	if strings.HasPrefix(lowerHeader, "cf-") {
		return true
	}
	switch lowerHeader {
	case "cdn-loop", "true-client-ip", "x-forwarded-for", "x-forwarded-proto", "x-real-ip":
		return true
	default:
		return false
	}
}

const (
	weeklyWindowSeconds = 604800
	weeklyWindowMinutes = weeklyWindowSeconds / 60
)

func extractWeeklyCycleHint(headers http.Header) *WeeklyCycleHint {
	resetAt, exists := parseOptionalAnthropicResetHeader(headers, "anthropic-ratelimit-unified-7d-reset")
	if !exists {
		return nil
	}

	return &WeeklyCycleHint{
		WindowMinutes: weeklyWindowMinutes,
		ResetAt:       resetAt.UTC(),
	}
}

type Service struct {
	boxService.Adapter
	ctx           context.Context
	logger        log.ContextLogger
	options       option.CCMServiceOptions
	httpHeaders   http.Header
	listener      *listener.Listener
	tlsConfig     tls.ServerConfig
	httpServer    *http.Server
	userManager   *UserManager
	trackingGroup sync.WaitGroup
	shuttingDown  bool

	// Legacy mode (single credential)
	legacyCredential *defaultCredential
	legacyProvider   credentialProvider

	// Multi-credential mode
	providers      map[string]credentialProvider
	allCredentials []credential
	userConfigMap  map[string]*option.CCMUser
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.CCMServiceOptions) (adapter.Service, error) {
	initCCMUserAgent(logger)

	err := validateCCMOptions(options)
	if err != nil {
		return nil, E.Cause(err, "validate options")
	}

	userManager := &UserManager{
		tokenMap: make(map[string]string),
	}

	service := &Service{
		Adapter:     boxService.NewAdapter(C.TypeCCM, tag),
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
		userManager: userManager,
	}

	if len(options.Credentials) > 0 {
		providers, allCredentials, err := buildCredentialProviders(ctx, options, logger)
		if err != nil {
			return nil, E.Cause(err, "build credential providers")
		}
		service.providers = providers
		service.allCredentials = allCredentials

		userConfigMap := make(map[string]*option.CCMUser)
		for i := range options.Users {
			userConfigMap[options.Users[i].Name] = &options.Users[i]
		}
		service.userConfigMap = userConfigMap
	} else {
		cred, err := newDefaultCredential(ctx, "default", option.CCMDefaultCredentialOptions{
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
		if extCred, ok := cred.(*externalCredential); ok && extCred.reverse && extCred.connectorURL != nil {
			extCred.reverseService = s
		}
		err := cred.start()
		if err != nil {
			return err
		}
	}

	router := chi.NewRouter()
	router.Mount("/", s)

	s.httpServer = &http.Server{Handler: h2c.NewHandler(router, &http2.Server{})}

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

func isExtendedContextRequest(betaHeader string) bool {
	for _, feature := range strings.Split(betaHeader, ",") {
		if strings.HasPrefix(strings.TrimSpace(feature), "context-1m") {
			return true
		}
	}
	return false
}

func isFastModeRequest(betaHeader string) bool {
	for _, feature := range strings.Split(betaHeader, ",") {
		if strings.HasPrefix(strings.TrimSpace(feature), "fast-mode") {
			return true
		}
	}
	return false
}

func detectContextWindow(betaHeader string, totalInputTokens int64) int {
	if totalInputTokens > premiumContextThreshold {
		if isExtendedContextRequest(betaHeader) {
			return contextWindowPremium
		}
	}
	return contextWindowStandard
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := log.ContextWithNewID(r.Context())
	if r.URL.Path == "/ccm/v1/status" {
		s.handleStatusEndpoint(w, r)
		return
	}

	if r.URL.Path == "/ccm/v1/reverse" {
		s.handleReverseConnect(ctx, w, r)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/v1/") {
		writeJSONError(w, r, http.StatusNotFound, "not_found_error", "Not found")
		return
	}

	var username string
	if len(s.options.Users) > 0 {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.logger.WarnContext(ctx, "authentication failed for request from ", r.RemoteAddr, ": missing Authorization header")
			writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "missing api key")
			return
		}
		clientToken := strings.TrimPrefix(authHeader, "Bearer ")
		if clientToken == authHeader {
			s.logger.WarnContext(ctx, "authentication failed for request from ", r.RemoteAddr, ": invalid Authorization format")
			writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "invalid api key format")
			return
		}
		var ok bool
		username, ok = s.userManager.Authenticate(clientToken)
		if !ok {
			s.logger.WarnContext(ctx, "authentication failed for request from ", r.RemoteAddr, ": unknown key: ", clientToken)
			writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "invalid api key")
			return
		}
	}

	// Always read body to extract model and session ID
	var bodyBytes []byte
	var requestModel string
	var messagesCount int
	var sessionID string

	if r.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			s.logger.ErrorContext(ctx, "read request body: ", err)
			writeJSONError(w, r, http.StatusInternalServerError, "api_error", "failed to read request body")
			return
		}

		var request struct {
			Model    string                   `json:"model"`
			Messages []anthropic.MessageParam `json:"messages"`
		}
		err = json.Unmarshal(bodyBytes, &request)
		if err == nil {
			requestModel = request.Model
			messagesCount = len(request.Messages)
		}

		sessionID = extractCCMSessionID(bodyBytes)
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Resolve credential provider and user config
	var provider credentialProvider
	var userConfig *option.CCMUser
	if len(s.options.Users) > 0 {
		userConfig = s.userConfigMap[username]
		var err error
		provider, err = credentialForUser(s.userConfigMap, s.providers, s.legacyProvider, username)
		if err != nil {
			s.logger.ErrorContext(ctx, "resolve credential: ", err)
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

	anthropicBetaHeader := r.Header.Get("anthropic-beta")
	if isFastModeRequest(anthropicBetaHeader) {
		if _, isSingle := provider.(*singleCredentialProvider); !isSingle {
			writeJSONError(w, r, http.StatusBadRequest, "invalid_request_error",
				"fast mode requests will consume Extra usage, please use a default credential directly")
			return
		}
	}

	var credentialFilter func(credential) bool
	if userConfig != nil && !userConfig.AllowExternalUsage {
		credentialFilter = func(c credential) bool { return !c.isExternal() }
	}

	selectedCredential, isNew, err := provider.selectCredential(sessionID, credentialFilter)
	if err != nil {
		writeNonRetryableCredentialError(w, r, unavailableCredentialMessage(provider, err.Error()))
		return
	}
	if isNew {
		logParts := []any{"assigned credential ", selectedCredential.tagName()}
		if sessionID != "" {
			logParts = append(logParts, " for session ", sessionID)
		}
		if username != "" {
			logParts = append(logParts, " by user ", username)
		}
		if requestModel != "" {
			modelDisplay := requestModel
			if isExtendedContextRequest(anthropicBetaHeader) {
				modelDisplay += "[1m]"
			}
			logParts = append(logParts, ", model=", modelDisplay)
		}
		s.logger.DebugContext(ctx, logParts...)
	}

	if isFastModeRequest(anthropicBetaHeader) && selectedCredential.isExternal() {
		writeJSONError(w, r, http.StatusBadRequest, "invalid_request_error",
			"fast mode requests cannot be proxied through external credentials")
		return
	}

	requestContext := selectedCredential.wrapRequestContext(ctx)
	provider.wrapProviderInterrupt(selectedCredential, requestContext)
	defer func() {
		requestContext.cancelRequest()
	}()
	proxyRequest, err := selectedCredential.buildProxyRequest(requestContext, r, bodyBytes, s.httpHeaders)
	if err != nil {
		s.logger.ErrorContext(ctx, "create proxy request: ", err)
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
		resetAt := parseRateLimitResetFromHeaders(response.Header)
		nextCredential := provider.onRateLimited(sessionID, selectedCredential, resetAt, credentialFilter)
		selectedCredential.updateStateFromHeaders(response.Header)
		if bodyBytes == nil || nextCredential == nil {
			response.Body.Close()
			writeCredentialUnavailableError(w, r, provider, selectedCredential, credentialFilter, "all credentials rate-limited")
			return
		}
		response.Body.Close()
		s.logger.InfoContext(ctx, "retrying with credential ", nextCredential.tagName(), " after 429 from ", selectedCredential.tagName())
		requestContext.cancelRequest()
		requestContext = nextCredential.wrapRequestContext(ctx)
		provider.wrapProviderInterrupt(nextCredential, requestContext)
		retryRequest, buildErr := nextCredential.buildProxyRequest(requestContext, r, bodyBytes, s.httpHeaders)
		if buildErr != nil {
			s.logger.ErrorContext(ctx, "retry request: ", buildErr)
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
			s.logger.ErrorContext(ctx, "retry request: ", retryErr)
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
		s.logger.ErrorContext(ctx, "upstream error from ", selectedCredential.tagName(), ": status ", response.StatusCode, " ", string(body))
		go selectedCredential.pollUsage(s.ctx)
		writeJSONError(w, r, http.StatusInternalServerError, "api_error",
			"proxy request (status "+strconv.Itoa(response.StatusCode)+"): "+string(body))
		return
	}

	// Rewrite response headers for external users
	if userConfig != nil && userConfig.ExternalCredential != "" {
		s.rewriteResponseHeadersForExternalUser(response.Header, userConfig)
	}

	for key, values := range response.Header {
		if !isHopByHopHeader(key) && !isReverseProxyHeader(key) {
			w.Header()[key] = values
		}
	}
	w.WriteHeader(response.StatusCode)

	usageTracker := selectedCredential.usageTrackerOrNil()
	if usageTracker != nil && response.StatusCode == http.StatusOK {
		s.handleResponseWithTracking(ctx, w, response, usageTracker, requestModel, anthropicBetaHeader, messagesCount, username)
	} else {
		mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
		if err == nil && mediaType != "text/event-stream" {
			_, _ = io.Copy(w, response.Body)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			s.logger.ErrorContext(ctx, "streaming not supported")
			return
		}
		buffer := make([]byte, buf.BufferSize)
		for {
			n, err := response.Body.Read(buffer)
			if n > 0 {
				_, writeError := w.Write(buffer[:n])
				if writeError != nil {
					s.logger.ErrorContext(ctx, "write streaming response: ", writeError)
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

func (s *Service) handleResponseWithTracking(ctx context.Context, writer http.ResponseWriter, response *http.Response, usageTracker *AggregatedUsage, requestModel string, anthropicBetaHeader string, messagesCount int, username string) {
	weeklyCycleHint := extractWeeklyCycleHint(response.Header)
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	isStreaming := err == nil && mediaType == "text/event-stream"

	if !isStreaming {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			s.logger.ErrorContext(ctx, "read response body: ", err)
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
				totalInputTokens := usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
				contextWindow := detectContextWindow(anthropicBetaHeader, totalInputTokens)
				usageTracker.AddUsageWithCycleHint(
					responseModel,
					contextWindow,
					messagesCount,
					usage.InputTokens,
					usage.OutputTokens,
					usage.CacheReadInputTokens,
					usage.CacheCreationInputTokens,
					usage.CacheCreation.Ephemeral5mInputTokens,
					usage.CacheCreation.Ephemeral1hInputTokens,
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
		s.logger.ErrorContext(ctx, "streaming not supported")
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
							accumulatedUsage.CacheCreation.Ephemeral5mInputTokens = messageStart.Message.Usage.CacheCreation.Ephemeral5mInputTokens
							accumulatedUsage.CacheCreation.Ephemeral1hInputTokens = messageStart.Message.Usage.CacheCreation.Ephemeral1hInputTokens
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
				s.logger.ErrorContext(ctx, "write streaming response: ", writeError)
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
					totalInputTokens := accumulatedUsage.InputTokens + accumulatedUsage.CacheCreationInputTokens + accumulatedUsage.CacheReadInputTokens
					contextWindow := detectContextWindow(anthropicBetaHeader, totalInputTokens)
					usageTracker.AddUsageWithCycleHint(
						responseModel,
						contextWindow,
						messagesCount,
						accumulatedUsage.InputTokens,
						accumulatedUsage.OutputTokens,
						accumulatedUsage.CacheReadInputTokens,
						accumulatedUsage.CacheCreationInputTokens,
						accumulatedUsage.CacheCreation.Ephemeral5mInputTokens,
						accumulatedUsage.CacheCreation.Ephemeral1hInputTokens,
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
	avgFiveHour, avgWeekly, totalWeight := s.computeAggregatedUtilization(provider, userConfig)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]float64{
		"five_hour_utilization": avgFiveHour,
		"weekly_utilization":    avgWeekly,
		"plan_weight":           totalWeight,
	})
}

func (s *Service) computeAggregatedUtilization(provider credentialProvider, userConfig *option.CCMUser) (float64, float64, float64) {
	var totalWeightedRemaining5h, totalWeightedRemainingWeekly, totalWeight float64
	for _, cred := range provider.allCredentials() {
		if !cred.isAvailable() {
			continue
		}
		if userConfig.ExternalCredential != "" && cred.tagName() == userConfig.ExternalCredential {
			continue
		}
		if !userConfig.AllowExternalUsage && cred.isExternal() {
			continue
		}
		weight := cred.planWeight()
		remaining5h := cred.fiveHourCap() - cred.fiveHourUtilization()
		if remaining5h < 0 {
			remaining5h = 0
		}
		remainingWeekly := cred.weeklyCap() - cred.weeklyUtilization()
		if remainingWeekly < 0 {
			remainingWeekly = 0
		}
		totalWeightedRemaining5h += remaining5h * weight
		totalWeightedRemainingWeekly += remainingWeekly * weight
		totalWeight += weight
	}
	if totalWeight == 0 {
		return 100, 100, 0
	}
	return 100 - totalWeightedRemaining5h/totalWeight,
		100 - totalWeightedRemainingWeekly/totalWeight,
		totalWeight
}

func (s *Service) rewriteResponseHeadersForExternalUser(headers http.Header, userConfig *option.CCMUser) {
	provider, err := credentialForUser(s.userConfigMap, s.providers, s.legacyProvider, userConfig.Name)
	if err != nil {
		return
	}

	avgFiveHour, avgWeekly, totalWeight := s.computeAggregatedUtilization(provider, userConfig)

	// Rewrite utilization headers to aggregated average (convert back to 0.0-1.0 range)
	headers.Set("anthropic-ratelimit-unified-5h-utilization", strconv.FormatFloat(avgFiveHour/100, 'f', 6, 64))
	headers.Set("anthropic-ratelimit-unified-7d-utilization", strconv.FormatFloat(avgWeekly/100, 'f', 6, 64))
	if totalWeight > 0 {
		headers.Set("X-CCM-Plan-Weight", strconv.FormatFloat(totalWeight, 'f', -1, 64))
	}
}

func (s *Service) InterfaceUpdated() {
	for _, cred := range s.allCredentials {
		extCred, ok := cred.(*externalCredential)
		if !ok {
			continue
		}
		if extCred.reverse && extCred.connectorURL != nil {
			extCred.reverseService = s
			extCred.resetReverseContext()
			go extCred.connectorLoop()
		}
	}
}

func (s *Service) Close() error {
	err := common.Close(
		common.PtrOrNil(s.httpServer),
		common.PtrOrNil(s.listener),
		s.tlsConfig,
	)

	for _, cred := range s.allCredentials {
		cred.close()
	}

	return err
}
