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
	clientConn    net.Conn
	upstreamConn  net.Conn
	credentialTag string
	closeOnce     sync.Once
}

func (s *webSocketSession) Close() {
	s.closeOnce.Do(func() {
		s.clientConn.Close()
		s.upstreamConn.Close()
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
	case strings.HasPrefix(lowerKey, "sec-websocket-"):
		return false
	default:
		return true
	}
}

func (s *Service) handleWebSocket(
	w http.ResponseWriter,
	r *http.Request,
	path string,
	username string,
	sessionID string,
	userConfig *option.OCMUser,
	provider credentialProvider,
	selectedCredential credential,
	credentialFilter func(credential) bool,
) {
	var (
		err                     error
		upstreamConn            net.Conn
		upstreamBufferedReader  *bufio.Reader
		upstreamResponseHeaders http.Header
		statusCode              int
	)

	for {
		accessToken, accessErr := selectedCredential.getAccessToken()
		if accessErr != nil {
			s.logger.Error("get access token for websocket: ", accessErr)
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

		upstreamResponseHeaders = make(http.Header)
		statusCode = 0
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
			},
			OnHeader: func(key, value []byte) error {
				upstreamResponseHeaders.Add(string(key), string(value))
				return nil
			},
		}

		upstreamConn, upstreamBufferedReader, _, err = upstreamDialer.Dial(s.ctx, upstreamURL)
		if err == nil {
			break
		}
		if statusCode == http.StatusTooManyRequests {
			resetAt := parseOCMRateLimitResetFromHeaders(upstreamResponseHeaders)
			nextCredential := provider.onRateLimited(sessionID, selectedCredential, resetAt, credentialFilter)
			if nextCredential == nil {
				selectedCredential.updateStateFromHeaders(upstreamResponseHeaders)
				writeCredentialUnavailableError(w, r, provider, selectedCredential, credentialFilter, "all credentials rate-limited")
				return
			}
			s.logger.Info("retrying websocket with credential ", nextCredential.tagName(), " after 429 from ", selectedCredential.tagName())
			selectedCredential = nextCredential
			continue
		}
		s.logger.Error("dial upstream websocket: ", err)
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
	clientConn, _, _, err := clientUpgrader.Upgrade(r, w)
	if err != nil {
		s.logger.Error("upgrade client websocket: ", err)
		upstreamConn.Close()
		return
	}
	session := &webSocketSession{
		clientConn:    clientConn,
		upstreamConn:  upstreamConn,
		credentialTag: selectedCredential.tagName(),
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
		s.proxyWebSocketClientToUpstream(clientConn, upstreamConn, selectedCredential, modelChannel)
	}()
	go func() {
		defer waitGroup.Done()
		defer session.Close()
		s.proxyWebSocketUpstreamToClient(upstreamReadWriter, clientConn, selectedCredential, modelChannel, username, weeklyCycleHint)
	}()
	waitGroup.Wait()
}

func (s *Service) proxyWebSocketClientToUpstream(clientConn net.Conn, upstreamConn net.Conn, selectedCredential credential, modelChannel chan<- string) {
	for {
		data, opCode, err := wsutil.ReadClientData(clientConn)
		if err != nil {
			if !E.IsClosedOrCanceled(err) {
				s.logger.Debug("read client websocket: ", err)
			}
			return
		}

		if opCode == ws.OpText && selectedCredential.usageTrackerOrNil() != nil {
			var request struct {
				Type  string `json:"type"`
				Model string `json:"model"`
			}
			if json.Unmarshal(data, &request) == nil && request.Type == "response.create" && request.Model != "" {
				select {
				case modelChannel <- request.Model:
				default:
				}
			}
		}

		err = wsutil.WriteClientMessage(upstreamConn, opCode, data)
		if err != nil {
			if !E.IsClosedOrCanceled(err) {
				s.logger.Debug("write upstream websocket: ", err)
			}
			return
		}
	}
}

func (s *Service) proxyWebSocketUpstreamToClient(upstreamReadWriter io.ReadWriter, clientConn net.Conn, selectedCredential credential, modelChannel <-chan string, username string, weeklyCycleHint *WeeklyCycleHint) {
	usageTracker := selectedCredential.usageTrackerOrNil()
	var requestModel string
	for {
		data, opCode, err := wsutil.ReadServerData(upstreamReadWriter)
		if err != nil {
			if !E.IsClosedOrCanceled(err) {
				s.logger.Debug("read upstream websocket: ", err)
			}
			return
		}

		if opCode == ws.OpText && usageTracker != nil {
			select {
			case model := <-modelChannel:
				requestModel = model
			default:
			}

			var event struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(data, &event) == nil && event.Type == "response.completed" {
				var streamEvent responses.ResponseStreamEventUnion
				if json.Unmarshal(data, &streamEvent) == nil {
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
			}
		}

		err = wsutil.WriteServerMessage(clientConn, opCode, data)
		if err != nil {
			if !E.IsClosedOrCanceled(err) {
				s.logger.Debug("write client websocket: ", err)
			}
			return
		}
	}
}
