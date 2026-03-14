package ocm

import (
	"bufio"
	"context"
	stdTLS "crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/ntp"
	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"

	"github.com/openai/openai-go/v3/responses"
)

type webSocketSession struct {
	clientConn               net.Conn
	upstreamConn             net.Conn
	credentialTag            string
	releaseProviderInterrupt func()
	closeOnce                sync.Once
}

func (s *webSocketSession) Close() {
	s.closeOnce.Do(func() {
		if s.releaseProviderInterrupt != nil {
			s.releaseProviderInterrupt()
		}
		if s.clientConn != nil {
			s.clientConn.Close()
		}
		if s.upstreamConn != nil {
			s.upstreamConn.Close()
		}
	})
}

func buildUpstreamWebSocketURL(baseURL string, proxyPath string) string {
	upstreamURL := baseURL
	if strings.HasPrefix(upstreamURL, "https://") {
		upstreamURL = "wss://" + upstreamURL[len("https://"):]
	} else if strings.HasPrefix(upstreamURL, "http://") {
		upstreamURL = "ws://" + upstreamURL[len("http://"):]
	}
	return upstreamURL + proxyPath
}

func isForwardableResponseHeader(key string) bool {
	lowerKey := strings.ToLower(key)
	switch {
	case strings.HasPrefix(lowerKey, "x-codex-"):
		return true
	case strings.HasPrefix(lowerKey, "x-reasoning"):
		return true
	case lowerKey == "openai-model":
		return true
	case strings.Contains(lowerKey, "-secondary-"):
		return true
	default:
		return false
	}
}

func isForwardableWebSocketRequestHeader(key string) bool {
	if isHopByHopHeader(key) || isReverseProxyHeader(key) {
		return false
	}

	lowerKey := strings.ToLower(key)
	switch {
	case lowerKey == "authorization":
		return false
	case lowerKey == "x-api-key" || lowerKey == "api-key":
		return false
	case strings.HasPrefix(lowerKey, "sec-websocket-"):
		return false
	default:
		return true
	}
}

func (s *Service) handleWebSocket(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	path string,
	username string,
	sessionID string,
	userConfig *option.OCMUser,
	provider credentialProvider,
	selectedCredential Credential,
	selection credentialSelection,
	isNew bool,
) {
	var (
		err                     error
		requestContext          *credentialRequestContext
		clientConn              net.Conn
		session                 *webSocketSession
		upstreamConn            net.Conn
		upstreamBufferedReader  *bufio.Reader
		upstreamResponseHeaders http.Header
		statusCode              int
		statusResponseBody      string
	)
	defer func() {
		if requestContext != nil {
			requestContext.cancelRequest()
		}
	}()

	for {
		accessToken, accessErr := selectedCredential.getAccessToken()
		if accessErr != nil {
			s.logger.ErrorContext(ctx, "get access token for websocket: ", accessErr)
			writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "authentication failed")
			return
		}

		var proxyPath string
		if selectedCredential.ocmIsAPIKeyMode() || selectedCredential.isExternal() {
			proxyPath = path
		} else {
			proxyPath = strings.TrimPrefix(path, "/v1")
		}

		upstreamURL := buildUpstreamWebSocketURL(selectedCredential.ocmGetBaseURL(), proxyPath)
		if r.URL.RawQuery != "" {
			upstreamURL += "?" + r.URL.RawQuery
		}

		upstreamHeaders := make(http.Header)
		for key, values := range r.Header {
			if isForwardableWebSocketRequestHeader(key) {
				upstreamHeaders[key] = values
			}
		}
		for key, values := range s.httpHeaders {
			upstreamHeaders.Del(key)
			upstreamHeaders[key] = values
		}
		upstreamHeaders.Set("Authorization", "Bearer "+accessToken)
		if accountID := selectedCredential.ocmGetAccountID(); accountID != "" {
			upstreamHeaders.Set("ChatGPT-Account-Id", accountID)
		}
		if upstreamHeaders.Get("OpenAI-Beta") == "" {
			upstreamHeaders.Set("OpenAI-Beta", "responses_websockets=2026-02-06")
		}

		upstreamResponseHeaders = make(http.Header)
		statusCode = 0
		statusResponseBody = ""
		upstreamDialer := ws.Dialer{
			NetDial: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return selectedCredential.ocmDialer().DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
			TLSConfig: &stdTLS.Config{
				RootCAs: adapter.RootPoolFromContext(s.ctx),
				Time:    ntp.TimeFuncFromContext(s.ctx),
			},
			Header: ws.HandshakeHeaderHTTP(upstreamHeaders),
			// gobwas/ws@v1.4.0: the response io.Reader is
			// MultiReader(statusLine_without_CRLF, "\r\n", bufferedConn).
			// ReadString('\n') consumes the status line, then ReadMIMEHeader
			// parses the remaining headers.
			OnStatusError: func(status int, reason []byte, response io.Reader) {
				statusCode = status
				bufferedResponse := bufio.NewReader(response)
				_, readErr := bufferedResponse.ReadString('\n')
				if readErr != nil {
					return
				}
				mimeHeader, readErr := textproto.NewReader(bufferedResponse).ReadMIMEHeader()
				if readErr == nil {
					upstreamResponseHeaders = http.Header(mimeHeader)
				}
				body, readErr := io.ReadAll(io.LimitReader(bufferedResponse, 4096))
				if readErr == nil && len(body) > 0 {
					statusResponseBody = string(body)
				}
			},
			OnHeader: func(key, value []byte) error {
				upstreamResponseHeaders.Add(string(key), string(value))
				return nil
			},
		}

		requestContext = selectedCredential.wrapRequestContext(ctx)
		{
			currentRequestContext := requestContext
			requestContext.addInterruptLink(provider.linkProviderInterrupt(selectedCredential, selection, func() {
				currentRequestContext.cancelOnce.Do(currentRequestContext.cancelFunc)
				if session != nil {
					session.Close()
					return
				}
				if clientConn != nil {
					clientConn.Close()
				}
				if upstreamConn != nil {
					upstreamConn.Close()
				}
			}))
		}
		upstreamConn, upstreamBufferedReader, _, err = upstreamDialer.Dial(requestContext, upstreamURL)
		if err == nil {
			break
		}
		requestContext.cancelRequest()
		requestContext = nil
		upstreamConn = nil
		clientConn = nil
		if statusCode == http.StatusTooManyRequests {
			resetAt := parseOCMRateLimitResetFromHeaders(upstreamResponseHeaders)
			nextCredential := provider.onRateLimited(sessionID, selectedCredential, resetAt, selection)
			selectedCredential.updateStateFromHeaders(upstreamResponseHeaders)
			if nextCredential == nil {
				writeCredentialUnavailableError(w, r, provider, selectedCredential, selection, "all credentials rate-limited")
				return
			}
			s.logger.InfoContext(ctx, "retrying websocket with credential ", nextCredential.tagName(), " after 429 from ", selectedCredential.tagName())
			selectedCredential = nextCredential
			continue
		}
		if statusCode > 0 && statusResponseBody != "" {
			s.logger.ErrorContext(ctx, "dial upstream websocket: status ", statusCode, " body: ", statusResponseBody)
		} else {
			s.logger.ErrorContext(ctx, "dial upstream websocket: ", err)
		}
		writeJSONError(w, r, http.StatusBadGateway, "api_error", "upstream websocket connection failed")
		return
	}

	selectedCredential.updateStateFromHeaders(upstreamResponseHeaders)
	weeklyCycleHint := extractWeeklyCycleHint(upstreamResponseHeaders)

	clientResponseHeaders := make(http.Header)
	for key, values := range upstreamResponseHeaders {
		if isForwardableResponseHeader(key) {
			clientResponseHeaders[key] = append([]string(nil), values...)
		}
	}
	if userConfig != nil && userConfig.ExternalCredential != "" {
		s.rewriteResponseHeadersForExternalUser(clientResponseHeaders, userConfig)
	}

	clientUpgrader := ws.HTTPUpgrader{
		Header: clientResponseHeaders,
	}
	if s.isShuttingDown() {
		upstreamConn.Close()
		writeJSONError(w, r, http.StatusServiceUnavailable, "api_error", "service is shutting down")
		return
	}
	clientConn, _, _, err = clientUpgrader.Upgrade(r, w)
	if err != nil {
		s.logger.ErrorContext(ctx, "upgrade client websocket: ", err)
		upstreamConn.Close()
		return
	}
	session = &webSocketSession{
		clientConn:               clientConn,
		upstreamConn:             upstreamConn,
		credentialTag:            selectedCredential.tagName(),
		releaseProviderInterrupt: requestContext.releaseCredentialInterrupt,
	}
	if !s.registerWebSocketSession(session) {
		session.Close()
		return
	}
	defer s.unregisterWebSocketSession(session)

	var upstreamReadWriter io.ReadWriter
	if upstreamBufferedReader != nil {
		upstreamReadWriter = struct {
			io.Reader
			io.Writer
		}{upstreamBufferedReader, upstreamConn}
	} else {
		upstreamReadWriter = upstreamConn
	}

	modelChannel := make(chan string, 1)
	var waitGroup sync.WaitGroup

	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		defer session.Close()
		s.proxyWebSocketClientToUpstream(ctx, clientConn, upstreamConn, selectedCredential, modelChannel, isNew, username, sessionID)
	}()
	go func() {
		defer waitGroup.Done()
		defer session.Close()
		s.proxyWebSocketUpstreamToClient(ctx, upstreamReadWriter, clientConn, selectedCredential, userConfig, provider, modelChannel, username, weeklyCycleHint)
	}()
	waitGroup.Wait()
}

func (s *Service) proxyWebSocketClientToUpstream(ctx context.Context, clientConn net.Conn, upstreamConn net.Conn, selectedCredential Credential, modelChannel chan<- string, isNew bool, username string, sessionID string) {
	logged := false
	for {
		data, opCode, err := wsutil.ReadClientData(clientConn)
		if err != nil {
			if !E.IsClosedOrCanceled(err) {
				s.logger.DebugContext(ctx, "read client websocket: ", err)
			}
			return
		}

		if opCode == ws.OpText {
			var request struct {
				Type        string `json:"type"`
				Model       string `json:"model"`
				ServiceTier string `json:"service_tier"`
			}
			if json.Unmarshal(data, &request) == nil && request.Type == "response.create" && request.Model != "" {
				if isNew && !logged {
					logged = true
					logParts := []any{"assigned credential ", selectedCredential.tagName()}
					if sessionID != "" {
						logParts = append(logParts, " for session ", sessionID)
					}
					if username != "" {
						logParts = append(logParts, " by user ", username)
					}
					logParts = append(logParts, ", model=", request.Model)
					if request.ServiceTier == "priority" {
						logParts = append(logParts, ", fast")
					}
					s.logger.DebugContext(ctx, logParts...)
				}
				if selectedCredential.usageTrackerOrNil() != nil {
					select {
					case modelChannel <- request.Model:
					default:
					}
				}
			}
		}

		err = wsutil.WriteClientMessage(upstreamConn, opCode, data)
		if err != nil {
			if !E.IsClosedOrCanceled(err) {
				s.logger.DebugContext(ctx, "write upstream websocket: ", err)
			}
			return
		}
	}
}

func (s *Service) proxyWebSocketUpstreamToClient(ctx context.Context, upstreamReadWriter io.ReadWriter, clientConn net.Conn, selectedCredential Credential, userConfig *option.OCMUser, provider credentialProvider, modelChannel <-chan string, username string, weeklyCycleHint *WeeklyCycleHint) {
	usageTracker := selectedCredential.usageTrackerOrNil()
	var requestModel string
	for {
		data, opCode, err := wsutil.ReadServerData(upstreamReadWriter)
		if err != nil {
			if !E.IsClosedOrCanceled(err) {
				s.logger.DebugContext(ctx, "read upstream websocket: ", err)
			}
			return
		}

		if opCode == ws.OpText {
			var event struct {
				Type       string `json:"type"`
				StatusCode int    `json:"status_code"`
			}
			if json.Unmarshal(data, &event) == nil {
				switch event.Type {
				case "codex.rate_limits":
					s.handleWebSocketRateLimitsEvent(data, selectedCredential)
					if userConfig != nil && userConfig.ExternalCredential != "" {
						rewritten, rewriteErr := s.rewriteWebSocketRateLimitsForExternalUser(data, provider, userConfig)
						if rewriteErr == nil {
							data = rewritten
						}
					}
				case "error":
					if event.StatusCode == http.StatusTooManyRequests {
						s.handleWebSocketErrorRateLimited(data, selectedCredential)
					}
				case "response.completed":
					if usageTracker != nil {
						select {
						case model := <-modelChannel:
							requestModel = model
						default:
						}
						s.handleWebSocketResponseCompleted(data, usageTracker, requestModel, username, weeklyCycleHint)
					}
				}
			}
		}

		err = wsutil.WriteServerMessage(clientConn, opCode, data)
		if err != nil {
			if !E.IsClosedOrCanceled(err) {
				s.logger.DebugContext(ctx, "write client websocket: ", err)
			}
			return
		}
	}
}

func (s *Service) handleWebSocketRateLimitsEvent(data []byte, selectedCredential Credential) {
	var rateLimitsEvent struct {
		RateLimits struct {
			Primary *struct {
				UsedPercent float64 `json:"used_percent"`
				ResetAt     int64   `json:"reset_at"`
			} `json:"primary"`
			Secondary *struct {
				UsedPercent float64 `json:"used_percent"`
				ResetAt     int64   `json:"reset_at"`
			} `json:"secondary"`
		} `json:"rate_limits"`
		LimitName        string  `json:"limit_name"`
		MeteredLimitName string  `json:"metered_limit_name"`
		PlanWeight       float64 `json:"plan_weight"`
	}
	err := json.Unmarshal(data, &rateLimitsEvent)
	if err != nil {
		return
	}
	identifier := rateLimitsEvent.MeteredLimitName
	if identifier == "" {
		identifier = rateLimitsEvent.LimitName
	}
	if identifier == "" {
		identifier = "codex"
	}
	identifier = normalizeRateLimitIdentifier(identifier)

	headers := make(http.Header)
	headers.Set("x-codex-active-limit", identifier)
	if w := rateLimitsEvent.RateLimits.Primary; w != nil {
		headers.Set("x-"+identifier+"-primary-used-percent", strconv.FormatFloat(w.UsedPercent, 'f', -1, 64))
		if w.ResetAt > 0 {
			headers.Set("x-"+identifier+"-primary-reset-at", strconv.FormatInt(w.ResetAt, 10))
		}
	}
	if w := rateLimitsEvent.RateLimits.Secondary; w != nil {
		headers.Set("x-"+identifier+"-secondary-used-percent", strconv.FormatFloat(w.UsedPercent, 'f', -1, 64))
		if w.ResetAt > 0 {
			headers.Set("x-"+identifier+"-secondary-reset-at", strconv.FormatInt(w.ResetAt, 10))
		}
	}
	if rateLimitsEvent.PlanWeight > 0 {
		headers.Set("X-OCM-Plan-Weight", strconv.FormatFloat(rateLimitsEvent.PlanWeight, 'f', -1, 64))
	}
	selectedCredential.updateStateFromHeaders(headers)
}

func (s *Service) handleWebSocketErrorRateLimited(data []byte, selectedCredential Credential) {
	var errorEvent struct {
		Headers map[string]string `json:"headers"`
	}
	err := json.Unmarshal(data, &errorEvent)
	if err != nil {
		return
	}
	headers := make(http.Header)
	for key, value := range errorEvent.Headers {
		headers.Set(key, value)
	}
	selectedCredential.updateStateFromHeaders(headers)
	resetAt := parseOCMRateLimitResetFromHeaders(headers)
	selectedCredential.markRateLimited(resetAt)
}

func (s *Service) rewriteWebSocketRateLimitsForExternalUser(data []byte, provider credentialProvider, userConfig *option.OCMUser) ([]byte, error) {
	var event map[string]json.RawMessage
	err := json.Unmarshal(data, &event)
	if err != nil {
		return nil, err
	}

	rateLimitsData, exists := event["rate_limits"]
	if !exists || len(rateLimitsData) == 0 || string(rateLimitsData) == "null" {
		return data, nil
	}

	var rateLimits map[string]json.RawMessage
	err = json.Unmarshal(rateLimitsData, &rateLimits)
	if err != nil {
		return nil, err
	}

	averageFiveHour, averageWeekly, totalWeight := s.computeAggregatedUtilization(provider, userConfig)

	if totalWeight > 0 {
		event["plan_weight"], _ = json.Marshal(totalWeight)
	}

	primaryData, err := rewriteWebSocketRateLimitWindow(rateLimits["primary"], averageFiveHour)
	if err != nil {
		return nil, err
	}
	if primaryData != nil {
		rateLimits["primary"] = primaryData
	}

	secondaryData, err := rewriteWebSocketRateLimitWindow(rateLimits["secondary"], averageWeekly)
	if err != nil {
		return nil, err
	}
	if secondaryData != nil {
		rateLimits["secondary"] = secondaryData
	}

	event["rate_limits"], err = json.Marshal(rateLimits)
	if err != nil {
		return nil, err
	}

	return json.Marshal(event)
}

func rewriteWebSocketRateLimitWindow(data json.RawMessage, usedPercent float64) (json.RawMessage, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}

	var window map[string]json.RawMessage
	err := json.Unmarshal(data, &window)
	if err != nil {
		return nil, err
	}

	window["used_percent"], err = json.Marshal(usedPercent)
	if err != nil {
		return nil, err
	}

	return json.Marshal(window)
}

func (s *Service) handleWebSocketResponseCompleted(data []byte, usageTracker *AggregatedUsage, requestModel string, username string, weeklyCycleHint *WeeklyCycleHint) {
	var streamEvent responses.ResponseStreamEventUnion
	if json.Unmarshal(data, &streamEvent) != nil {
		return
	}
	completedEvent := streamEvent.AsResponseCompleted()
	responseModel := string(completedEvent.Response.Model)
	serviceTier := string(completedEvent.Response.ServiceTier)
	inputTokens := completedEvent.Response.Usage.InputTokens
	outputTokens := completedEvent.Response.Usage.OutputTokens
	cachedTokens := completedEvent.Response.Usage.InputTokensDetails.CachedTokens

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
}
