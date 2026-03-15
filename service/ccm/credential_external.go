package ccm

import (
	"bytes"
	"context"
	stdTLS "crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"

	"github.com/hashicorp/yamux"
)

const reverseProxyBaseURL = "http://reverse-proxy"

type externalCredential struct {
	tag               string
	baseURL           string
	token             string
	forwardHTTPClient *http.Client
	state             credentialState
	stateAccess       sync.RWMutex
	pollAccess        sync.Mutex
	pollInterval      time.Duration
	usageTracker      *AggregatedUsage
	logger            log.ContextLogger

	interrupted    bool
	requestContext   context.Context
	cancelRequests   context.CancelFunc
	requestAccess    sync.Mutex

	// Reverse proxy fields
	reverse              bool
	reverseHTTPClient    *http.Client
	reverseSession       *yamux.Session
	reverseAccess        sync.RWMutex
	closed               bool
	reverseContext       context.Context
	reverseCancel        context.CancelFunc
	connectorDialer      N.Dialer
	connectorDestination M.Socksaddr
	connectorRequestPath string
	connectorURL         *url.URL
	connectorTLS         *stdTLS.Config
	reverseService       http.Handler
}

func externalCredentialURLPort(parsedURL *url.URL) uint16 {
	portString := parsedURL.Port()
	if portString != "" {
		port, err := strconv.ParseUint(portString, 10, 16)
		if err == nil {
			return uint16(port)
		}
	}
	if parsedURL.Scheme == "https" {
		return 443
	}
	return 80
}

func externalCredentialServerPort(parsedURL *url.URL, configuredPort uint16) uint16 {
	if configuredPort != 0 {
		return configuredPort
	}
	return externalCredentialURLPort(parsedURL)
}

func externalCredentialBaseURL(parsedURL *url.URL) string {
	baseURL := parsedURL.Scheme + "://" + parsedURL.Host
	if parsedURL.Path != "" && parsedURL.Path != "/" {
		baseURL += parsedURL.Path
	}
	if len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	return baseURL
}

func externalCredentialReversePath(parsedURL *url.URL, endpointPath string) string {
	pathPrefix := parsedURL.EscapedPath()
	if pathPrefix == "/" {
		pathPrefix = ""
	} else {
		pathPrefix = strings.TrimSuffix(pathPrefix, "/")
	}
	return pathPrefix + endpointPath
}

func newExternalCredential(ctx context.Context, tag string, options option.CCMExternalCredentialOptions, logger log.ContextLogger) (*externalCredential, error) {
	pollInterval := time.Duration(options.PollInterval)
	if pollInterval <= 0 {
		pollInterval = 30 * time.Minute
	}

	requestContext, cancelRequests := context.WithCancel(context.Background())
	reverseContext, reverseCancel := context.WithCancel(context.Background())

	credential := &externalCredential{
		tag:            tag,
		token:          options.Token,
		pollInterval:   pollInterval,
		logger:         logger,
		requestContext: requestContext,
		cancelRequests: cancelRequests,
		reverse:        options.Reverse,
		reverseContext: reverseContext,
		reverseCancel:  reverseCancel,
	}

	if options.URL == "" {
		// Receiver mode: no URL, wait for reverse connection
		credential.baseURL = reverseProxyBaseURL
		credential.forwardHTTPClient = &http.Client{
			Transport: &http.Transport{
				ForceAttemptHTTP2: false,
				DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
					return credential.openReverseConnection(ctx)
				},
			},
		}
	} else {
		// Normal or connector mode: has URL
		parsedURL, err := url.Parse(options.URL)
		if err != nil {
			return nil, E.Cause(err, "parse url for credential ", tag)
		}

		credentialDialer, err := dialer.NewWithOptions(dialer.Options{
			Context: ctx,
			Options: option.DialerOptions{
				Detour: options.Detour,
			},
			RemoteIsDomain: true,
		})
		if err != nil {
			return nil, E.Cause(err, "create dialer for credential ", tag)
		}

		transport := &http.Transport{
			ForceAttemptHTTP2: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if options.Server != "" {
					destination := M.ParseSocksaddrHostPort(options.Server, externalCredentialServerPort(parsedURL, options.ServerPort))
					return credentialDialer.DialContext(ctx, network, destination)
				}
				return credentialDialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		}

		if parsedURL.Scheme == "https" {
			transport.TLSClientConfig = &stdTLS.Config{
				ServerName: parsedURL.Hostname(),
				RootCAs:    adapter.RootPoolFromContext(ctx),
				Time:       ntp.TimeFuncFromContext(ctx),
			}
		}

		credential.baseURL = externalCredentialBaseURL(parsedURL)

		if options.Reverse {
			// Connector mode: we dial out to serve, not to proxy
			credential.connectorDialer = credentialDialer
			if options.Server != "" {
				credential.connectorDestination = M.ParseSocksaddrHostPort(options.Server, externalCredentialServerPort(parsedURL, options.ServerPort))
			} else {
				credential.connectorDestination = M.ParseSocksaddrHostPort(parsedURL.Hostname(), externalCredentialURLPort(parsedURL))
			}
			credential.connectorRequestPath = externalCredentialReversePath(parsedURL, "/ccm/v1/reverse")
			credential.connectorURL = parsedURL
			if parsedURL.Scheme == "https" {
				credential.connectorTLS = &stdTLS.Config{
					ServerName: parsedURL.Hostname(),
					RootCAs:    adapter.RootPoolFromContext(ctx),
					Time:       ntp.TimeFuncFromContext(ctx),
				}
			}
			credential.forwardHTTPClient = &http.Client{Transport: transport}
		} else {
			// Normal mode: standard HTTP client for proxying
			credential.forwardHTTPClient = &http.Client{Transport: transport}
			credential.reverseHTTPClient = &http.Client{
				Transport: &http.Transport{
					ForceAttemptHTTP2: false,
					DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
						return credential.openReverseConnection(ctx)
					},
				},
			}
		}
	}

	if options.UsagesPath != "" {
		credential.usageTracker = &AggregatedUsage{
			LastUpdated:  time.Now(),
			Combinations: make([]CostCombination, 0),
			filePath:     options.UsagesPath,
			logger:       logger,
		}
	}

	return credential, nil
}

func (c *externalCredential) start() error {
	if c.usageTracker != nil {
		err := c.usageTracker.Load()
		if err != nil {
			c.logger.Warn("load usage statistics for ", c.tag, ": ", err)
		}
	}
	if c.reverse && c.connectorURL != nil {
		go c.connectorLoop()
	}
	return nil
}

func (c *externalCredential) tagName() string {
	return c.tag
}

func (c *externalCredential) isExternal() bool {
	return true
}

func (c *externalCredential) isAvailable() bool {
	return c.unavailableError() == nil
}

func (c *externalCredential) isUsable() bool {
	if !c.isAvailable() {
		return false
	}
	c.stateAccess.RLock()
	if c.state.consecutivePollFailures > 0 {
		c.stateAccess.RUnlock()
		return false
	}
	if c.state.hardRateLimited {
		if time.Now().Before(c.state.rateLimitResetAt) {
			c.stateAccess.RUnlock()
			return false
		}
		c.stateAccess.RUnlock()
		c.stateAccess.Lock()
		if c.state.hardRateLimited && !time.Now().Before(c.state.rateLimitResetAt) {
			c.state.hardRateLimited = false
		}
		// No reserve for external: only 100% is unusable
		usable := c.state.fiveHourUtilization < 100 && c.state.weeklyUtilization < 100
		c.stateAccess.Unlock()
		return usable
	}
	usable := c.state.fiveHourUtilization < 100 && c.state.weeklyUtilization < 100
	c.stateAccess.RUnlock()
	return usable
}

func (c *externalCredential) fiveHourUtilization() float64 {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	return c.state.fiveHourUtilization
}

func (c *externalCredential) weeklyUtilization() float64 {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	return c.state.weeklyUtilization
}

func (c *externalCredential) fiveHourCap() float64 {
	return 100
}

func (c *externalCredential) weeklyCap() float64 {
	return 100
}

func (c *externalCredential) planWeight() float64 {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	if c.state.remotePlanWeight > 0 {
		return c.state.remotePlanWeight
	}
	return 10
}

func (c *externalCredential) weeklyResetTime() time.Time {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	return c.state.weeklyReset
}

func (c *externalCredential) markRateLimited(resetAt time.Time) {
	c.logger.Warn("rate limited for ", c.tag, ", reset in ", log.FormatDuration(time.Until(resetAt)))
	c.stateAccess.Lock()
	c.state.hardRateLimited = true
	c.state.rateLimitResetAt = resetAt
	shouldInterrupt := c.checkTransitionLocked()
	c.stateAccess.Unlock()
	if shouldInterrupt {
		c.interruptConnections()
	}
}

func (c *externalCredential) earliestReset() time.Time {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	if c.state.hardRateLimited {
		return c.state.rateLimitResetAt
	}
	earliest := c.state.fiveHourReset
	if !c.state.weeklyReset.IsZero() && (earliest.IsZero() || c.state.weeklyReset.Before(earliest)) {
		earliest = c.state.weeklyReset
	}
	return earliest
}

func (c *externalCredential) unavailableError() error {
	if c.reverse && c.connectorURL != nil {
		return E.New("credential ", c.tag, " is unavailable: reverse connector credentials cannot serve local requests")
	}
	if c.baseURL == reverseProxyBaseURL {
		session := c.getReverseSession()
		if session == nil || session.IsClosed() {
			return E.New("credential ", c.tag, " is unavailable: reverse connection not established")
		}
	}
	return nil
}

func (c *externalCredential) getAccessToken() (string, error) {
	return c.token, nil
}

func (c *externalCredential) buildProxyRequest(ctx context.Context, original *http.Request, bodyBytes []byte, _ http.Header) (*http.Request, error) {
	baseURL := c.baseURL
	if c.reverseHTTPClient != nil {
		session := c.getReverseSession()
		if session != nil && !session.IsClosed() {
			baseURL = reverseProxyBaseURL
		}
	}
	proxyURL := baseURL + original.URL.RequestURI()
	var body io.Reader
	if bodyBytes != nil {
		body = bytes.NewReader(bodyBytes)
	} else {
		body = original.Body
	}
	proxyRequest, err := http.NewRequestWithContext(ctx, original.Method, proxyURL, body)
	if err != nil {
		return nil, err
	}

	for key, values := range original.Header {
		if !isHopByHopHeader(key) && !isReverseProxyHeader(key) && !isAPIKeyHeader(key) && key != "Authorization" {
			proxyRequest.Header[key] = values
		}
	}

	proxyRequest.Header.Set("Authorization", "Bearer "+c.token)

	return proxyRequest, nil
}

func (c *externalCredential) openReverseConnection(ctx context.Context) (net.Conn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	session := c.getReverseSession()
	if session == nil || session.IsClosed() {
		return nil, E.New("reverse connection not established for ", c.tag)
	}
	conn, err := session.Open()
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		conn.Close()
		return nil, ctx.Err()
	default:
	}
	return conn, nil
}

func (c *externalCredential) updateStateFromHeaders(headers http.Header) {
	c.stateAccess.Lock()
	isFirstUpdate := c.state.lastUpdated.IsZero()
	oldFiveHour := c.state.fiveHourUtilization
	oldWeekly := c.state.weeklyUtilization
	hadData := false

	if value, exists := parseOptionalAnthropicResetHeader(headers, "anthropic-ratelimit-unified-5h-reset"); exists {
		hadData = true
		c.state.fiveHourReset = value
	}
	if utilization := headers.Get("anthropic-ratelimit-unified-5h-utilization"); utilization != "" {
		value, err := strconv.ParseFloat(utilization, 64)
		if err == nil {
			hadData = true
			c.state.fiveHourUtilization = value * 100
		}
	}

	if value, exists := parseOptionalAnthropicResetHeader(headers, "anthropic-ratelimit-unified-7d-reset"); exists {
		hadData = true
		c.state.weeklyReset = value
	}
	if utilization := headers.Get("anthropic-ratelimit-unified-7d-utilization"); utilization != "" {
		value, err := strconv.ParseFloat(utilization, 64)
		if err == nil {
			hadData = true
			c.state.weeklyUtilization = value * 100
		}
	}
	if planWeight := headers.Get("X-CCM-Plan-Weight"); planWeight != "" {
		value, err := strconv.ParseFloat(planWeight, 64)
		if err == nil && value > 0 {
			c.state.remotePlanWeight = value
		}
	}
	if hadData {
		c.state.consecutivePollFailures = 0
		c.state.lastUpdated = time.Now()
	}
	if isFirstUpdate || int(c.state.fiveHourUtilization*100) != int(oldFiveHour*100) || int(c.state.weeklyUtilization*100) != int(oldWeekly*100) {
		resetSuffix := ""
		if !c.state.weeklyReset.IsZero() {
			resetSuffix = ", resets=" + log.FormatDuration(time.Until(c.state.weeklyReset))
		}
		c.logger.Debug("usage update for ", c.tag, ": 5h=", c.state.fiveHourUtilization, "%, weekly=", c.state.weeklyUtilization, "%", resetSuffix)
	}
	shouldInterrupt := c.checkTransitionLocked()
	c.stateAccess.Unlock()
	if shouldInterrupt {
		c.interruptConnections()
	}
}

func (c *externalCredential) checkTransitionLocked() bool {
	unusable := c.state.hardRateLimited || c.state.fiveHourUtilization >= 100 || c.state.weeklyUtilization >= 100 || c.state.consecutivePollFailures > 0
	if unusable && !c.interrupted {
		c.interrupted = true
		return true
	}
	if !unusable && c.interrupted {
		c.interrupted = false
	}
	return false
}

func (c *externalCredential) wrapRequestContext(parent context.Context) *credentialRequestContext {
	c.requestAccess.Lock()
	credentialContext := c.requestContext
	c.requestAccess.Unlock()
	derived, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(credentialContext, func() {
		cancel()
	})
	return &credentialRequestContext{
		Context:      derived,
		releaseFuncs: []func() bool{stop},
		cancelFunc:   cancel,
	}
}

func (c *externalCredential) interruptConnections() {
	c.logger.Warn("interrupting connections for ", c.tag)
	c.requestAccess.Lock()
	c.cancelRequests()
	c.requestContext, c.cancelRequests = context.WithCancel(context.Background())
	c.requestAccess.Unlock()
}

func (c *externalCredential) doPollUsageRequest(ctx context.Context) (*http.Response, error) {
	buildRequest := func(baseURL string) func() (*http.Request, error) {
		return func() (*http.Request, error) {
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/ccm/v1/status", nil)
			if err != nil {
				return nil, err
			}
			request.Header.Set("Authorization", "Bearer "+c.token)
			return request, nil
		}
	}
	// Try reverse transport first (single attempt, no retry)
	if c.reverseHTTPClient != nil {
		session := c.getReverseSession()
		if session != nil && !session.IsClosed() {
			request, err := buildRequest(reverseProxyBaseURL)()
			if err != nil {
				return nil, err
			}
			reverseClient := &http.Client{
				Transport: c.reverseHTTPClient.Transport,
				Timeout:   5 * time.Second,
			}
			response, err := reverseClient.Do(request)
			if err == nil {
				return response, nil
			}
			// Reverse failed, fall through to forward if available
		}
	}
	// Forward transport with retries
	if c.forwardHTTPClient != nil {
		forwardClient := &http.Client{
			Transport: c.forwardHTTPClient.Transport,
			Timeout:   5 * time.Second,
		}
		return doHTTPWithRetry(ctx, forwardClient, buildRequest(c.baseURL))
	}
	return nil, E.New("no transport available")
}

func (c *externalCredential) pollUsage(ctx context.Context) {
	if !c.pollAccess.TryLock() {
		return
	}
	defer c.pollAccess.Unlock()
	defer c.markUsagePollAttempted()

	response, err := c.doPollUsageRequest(ctx)
	if err != nil {
		c.logger.Debug("poll usage for ", c.tag, ": ", err)
		c.clearPollFailures()
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		c.logger.Debug("poll usage for ", c.tag, ": status ", response.StatusCode, " ", string(body))
		c.clearPollFailures()
		return
	}

	var statusResponse struct {
		FiveHourUtilization float64 `json:"five_hour_utilization"`
		WeeklyUtilization   float64 `json:"weekly_utilization"`
		PlanWeight          float64 `json:"plan_weight"`
	}
	err = json.NewDecoder(response.Body).Decode(&statusResponse)
	if err != nil {
		c.logger.Debug("poll usage for ", c.tag, ": decode: ", err)
		c.clearPollFailures()
		return
	}

	c.stateAccess.Lock()
	isFirstUpdate := c.state.lastUpdated.IsZero()
	oldFiveHour := c.state.fiveHourUtilization
	oldWeekly := c.state.weeklyUtilization
	c.state.consecutivePollFailures = 0
	c.state.fiveHourUtilization = statusResponse.FiveHourUtilization
	c.state.weeklyUtilization = statusResponse.WeeklyUtilization
	if statusResponse.PlanWeight > 0 {
		c.state.remotePlanWeight = statusResponse.PlanWeight
	}
	if c.state.hardRateLimited && time.Now().After(c.state.rateLimitResetAt) {
		c.state.hardRateLimited = false
	}
	if isFirstUpdate || int(c.state.fiveHourUtilization*100) != int(oldFiveHour*100) || int(c.state.weeklyUtilization*100) != int(oldWeekly*100) {
		resetSuffix := ""
		if !c.state.weeklyReset.IsZero() {
			resetSuffix = ", resets=" + log.FormatDuration(time.Until(c.state.weeklyReset))
		}
		c.logger.Debug("poll usage for ", c.tag, ": 5h=", c.state.fiveHourUtilization, "%, weekly=", c.state.weeklyUtilization, "%", resetSuffix)
	}
	shouldInterrupt := c.checkTransitionLocked()
	c.stateAccess.Unlock()
	if shouldInterrupt {
		c.interruptConnections()
	}
}

func (c *externalCredential) lastUpdatedTime() time.Time {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	return c.state.lastUpdated
}

func (c *externalCredential) markUsagePollAttempted() {
	c.stateAccess.Lock()
	defer c.stateAccess.Unlock()
	c.state.lastUpdated = time.Now()
}

func (c *externalCredential) pollBackoff(baseInterval time.Duration) time.Duration {
	return baseInterval
}

func (c *externalCredential) clearPollFailures() {
	c.stateAccess.Lock()
	c.state.consecutivePollFailures = 0
	c.checkTransitionLocked()
	c.stateAccess.Unlock()
}

func (c *externalCredential) usageTrackerOrNil() *AggregatedUsage {
	return c.usageTracker
}

func (c *externalCredential) httpClient() *http.Client {
	if c.reverseHTTPClient != nil {
		session := c.getReverseSession()
		if session != nil && !session.IsClosed() {
			return c.reverseHTTPClient
		}
	}
	return c.forwardHTTPClient
}

func (c *externalCredential) close() {
	var session *yamux.Session
	c.reverseAccess.Lock()
	if !c.closed {
		c.closed = true
		if c.reverseCancel != nil {
			c.reverseCancel()
		}
		session = c.reverseSession
		c.reverseSession = nil
	}
	c.reverseAccess.Unlock()
	if session != nil {
		session.Close()
	}
	if c.usageTracker != nil {
		c.usageTracker.cancelPendingSave()
		err := c.usageTracker.Save()
		if err != nil {
			c.logger.Error("save usage statistics for ", c.tag, ": ", err)
		}
	}
}

func (c *externalCredential) getReverseSession() *yamux.Session {
	c.reverseAccess.RLock()
	defer c.reverseAccess.RUnlock()
	return c.reverseSession
}

func (c *externalCredential) setReverseSession(session *yamux.Session) bool {
	c.reverseAccess.Lock()
	if c.closed {
		c.reverseAccess.Unlock()
		return false
	}
	old := c.reverseSession
	c.reverseSession = session
	c.reverseAccess.Unlock()
	if old != nil {
		old.Close()
	}
	return true
}

func (c *externalCredential) clearReverseSession(session *yamux.Session) {
	c.reverseAccess.Lock()
	if c.reverseSession == session {
		c.reverseSession = nil
	}
	c.reverseAccess.Unlock()
}

func (c *externalCredential) getReverseContext() context.Context {
	c.reverseAccess.RLock()
	defer c.reverseAccess.RUnlock()
	return c.reverseContext
}

func (c *externalCredential) resetReverseContext() {
	c.reverseAccess.Lock()
	if c.closed {
		c.reverseAccess.Unlock()
		return
	}
	c.reverseCancel()
	c.reverseContext, c.reverseCancel = context.WithCancel(context.Background())
	c.reverseAccess.Unlock()
}
