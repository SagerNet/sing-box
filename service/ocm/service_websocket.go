package ocm

import (
	"context"
	stdTLS "crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/ntp"
	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"

	"github.com/openai/openai-go/v3/responses"
)

type webSocketSession struct {
	clientConn   net.Conn
	upstreamConn net.Conn
	closeOnce    sync.Once
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
	if isHopByHopHeader(key) {
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

func (s *Service) handleWebSocket(w http.ResponseWriter, r *http.Request, proxyPath string, username string) {
	accessToken, err := s.getAccessToken()
	if err != nil {
		s.logger.Error("get access token for websocket: ", err)
		writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "authentication failed")
		return
	}

	upstreamURL := buildUpstreamWebSocketURL(s.getBaseURL(), proxyPath)
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
	if accountID := s.getAccountID(); accountID != "" {
		upstreamHeaders.Set("ChatGPT-Account-Id", accountID)
	}

	upstreamResponseHeaders := make(http.Header)
	upstreamDialer := ws.Dialer{
		NetDial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return s.dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
		},
		TLSConfig: &stdTLS.Config{
			RootCAs: adapter.RootPoolFromContext(s.ctx),
			Time:    ntp.TimeFuncFromContext(s.ctx),
		},
		Header: ws.HandshakeHeaderHTTP(upstreamHeaders),
		OnHeader: func(key, value []byte) error {
			upstreamResponseHeaders.Add(string(key), string(value))
			return nil
		},
	}

	upstreamConn, upstreamBufferedReader, _, err := upstreamDialer.Dial(r.Context(), upstreamURL)
	if err != nil {
		s.logger.Error("dial upstream websocket: ", err)
		writeJSONError(w, r, http.StatusBadGateway, "api_error", "upstream websocket connection failed")
		return
	}

	weeklyCycleHint := extractWeeklyCycleHint(upstreamResponseHeaders)

	clientResponseHeaders := make(http.Header)
	for key, values := range upstreamResponseHeaders {
		if isForwardableResponseHeader(key) {
			clientResponseHeaders[key] = values
		}
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
		clientConn:   clientConn,
		upstreamConn: upstreamConn,
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
		s.proxyWebSocketClientToUpstream(clientConn, upstreamConn, modelChannel)
	}()
	go func() {
		defer waitGroup.Done()
		defer session.Close()
		s.proxyWebSocketUpstreamToClient(upstreamReadWriter, clientConn, modelChannel, username, weeklyCycleHint)
	}()
	waitGroup.Wait()
}

func (s *Service) proxyWebSocketClientToUpstream(clientConn net.Conn, upstreamConn net.Conn, modelChannel chan<- string) {
	for {
		data, opCode, err := wsutil.ReadClientData(clientConn)
		if err != nil {
			if !E.IsClosedOrCanceled(err) {
				s.logger.Debug("read client websocket: ", err)
			}
			return
		}

		if opCode == ws.OpText && s.usageTracker != nil {
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

func (s *Service) proxyWebSocketUpstreamToClient(upstreamReadWriter io.ReadWriter, clientConn net.Conn, modelChannel <-chan string, username string, weeklyCycleHint *WeeklyCycleHint) {
	var requestModel string
	for {
		data, opCode, err := wsutil.ReadServerData(upstreamReadWriter)
		if err != nil {
			if !E.IsClosedOrCanceled(err) {
				s.logger.Debug("read upstream websocket: ", err)
			}
			return
		}

		if opCode == ws.OpText && s.usageTracker != nil {
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
							s.usageTracker.AddUsageWithCycleHint(
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
