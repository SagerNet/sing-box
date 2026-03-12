package ocm

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
)

type externalCredential struct {
	tag          string
	baseURL      string
	token        string
	credDialer   N.Dialer
	httpClient   *http.Client
	state        credentialState
	stateMutex   sync.RWMutex
	pollAccess   sync.Mutex
	pollInterval time.Duration
	usageTracker *AggregatedUsage
	logger       log.ContextLogger

	onBecameUnusable func()
	interrupted      bool
	requestContext   context.Context
	cancelRequests   context.CancelFunc
	requestAccess    sync.Mutex
}

func newExternalCredential(ctx context.Context, tag string, options option.OCMExternalCredentialOptions, logger log.ContextLogger) (*externalCredential, error) {
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
				serverPort := options.ServerPort
				if serverPort == 0 {
					portStr := parsedURL.Port()
					if portStr != "" {
						port, parseErr := strconv.ParseUint(portStr, 10, 16)
						if parseErr == nil {
							serverPort = uint16(port)
						}
					}
					if serverPort == 0 {
						if parsedURL.Scheme == "https" {
							serverPort = 443
						} else {
							serverPort = 80
						}
					}
				}
				destination := M.ParseSocksaddrHostPort(options.Server, serverPort)
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

	baseURL := parsedURL.Scheme + "://" + parsedURL.Host
	if parsedURL.Path != "" && parsedURL.Path != "/" {
		baseURL += parsedURL.Path
	}
	if len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	pollInterval := time.Duration(options.PollInterval)
	if pollInterval <= 0 {
		pollInterval = 30 * time.Minute
	}

	requestContext, cancelRequests := context.WithCancel(context.Background())

	cred := &externalCredential{
		tag:            tag,
		baseURL:        baseURL,
		token:          options.Token,
		credDialer:     credentialDialer,
		httpClient:     &http.Client{Transport: transport},
		pollInterval:   pollInterval,
		logger:         logger,
		requestContext: requestContext,
		cancelRequests: cancelRequests,
	}

	if options.UsagesPath != "" {
		cred.usageTracker = &AggregatedUsage{
			LastUpdated:  time.Now(),
			Combinations: make([]CostCombination, 0),
			filePath:     options.UsagesPath,
			logger:       logger,
		}
	}

	return cred, nil
}

func (c *externalCredential) start() error {
	if c.usageTracker != nil {
		err := c.usageTracker.Load()
		if err != nil {
			c.logger.Warn("load usage statistics for ", c.tag, ": ", err)
		}
	}
	return nil
}

func (c *externalCredential) setOnBecameUnusable(fn func()) {
	c.onBecameUnusable = fn
}

func (c *externalCredential) tagName() string {
	return c.tag
}

func (c *externalCredential) isExternal() bool {
	return true
}

func (c *externalCredential) isAvailable() bool {
	return true
}

func (c *externalCredential) isUsable() bool {
	c.stateMutex.RLock()
	if c.state.hardRateLimited {
		if time.Now().Before(c.state.rateLimitResetAt) {
			c.stateMutex.RUnlock()
			return false
		}
		c.stateMutex.RUnlock()
		c.stateMutex.Lock()
		if c.state.hardRateLimited && !time.Now().Before(c.state.rateLimitResetAt) {
			c.state.hardRateLimited = false
		}
		usable := c.state.fiveHourUtilization < 100 && c.state.weeklyUtilization < 100
		c.stateMutex.Unlock()
		return usable
	}
	usable := c.state.fiveHourUtilization < 100 && c.state.weeklyUtilization < 100
	c.stateMutex.RUnlock()
	return usable
}

func (c *externalCredential) fiveHourUtilization() float64 {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.state.fiveHourUtilization
}

func (c *externalCredential) weeklyUtilization() float64 {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.state.weeklyUtilization
}

func (c *externalCredential) markRateLimited(resetAt time.Time) {
	c.logger.Warn("rate limited for ", c.tag, ", reset in ", log.FormatDuration(time.Until(resetAt)))
	c.stateMutex.Lock()
	c.state.hardRateLimited = true
	c.state.rateLimitResetAt = resetAt
	shouldInterrupt := c.checkTransitionLocked()
	c.stateMutex.Unlock()
	if shouldInterrupt {
		c.interruptConnections()
	}
}

func (c *externalCredential) earliestReset() time.Time {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
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
	return nil
}

func (c *externalCredential) getAccessToken() (string, error) {
	return c.token, nil
}

func (c *externalCredential) buildProxyRequest(ctx context.Context, original *http.Request, bodyBytes []byte, _ http.Header) (*http.Request, error) {
	proxyURL := c.baseURL + original.URL.RequestURI()
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
		if !isHopByHopHeader(key) && key != "Authorization" {
			proxyRequest.Header[key] = values
		}
	}

	proxyRequest.Header.Set("Authorization", "Bearer "+c.token)

	return proxyRequest, nil
}

func (c *externalCredential) updateStateFromHeaders(headers http.Header) {
	c.stateMutex.Lock()
	isFirstUpdate := c.state.lastUpdated.IsZero()
	oldFiveHour := c.state.fiveHourUtilization
	oldWeekly := c.state.weeklyUtilization

	activeLimitIdentifier := normalizeRateLimitIdentifier(headers.Get("x-codex-active-limit"))
	if activeLimitIdentifier == "" {
		activeLimitIdentifier = "codex"
	}

	fiveHourPercent := headers.Get("x-" + activeLimitIdentifier + "-primary-used-percent")
	if fiveHourPercent != "" {
		value, err := strconv.ParseFloat(fiveHourPercent, 64)
		if err == nil {
			c.state.fiveHourUtilization = value
		}
	}
	fiveHourResetAt := headers.Get("x-" + activeLimitIdentifier + "-primary-reset-at")
	if fiveHourResetAt != "" {
		value, err := strconv.ParseInt(fiveHourResetAt, 10, 64)
		if err == nil {
			c.state.fiveHourReset = time.Unix(value, 0)
		}
	}
	weeklyPercent := headers.Get("x-" + activeLimitIdentifier + "-secondary-used-percent")
	if weeklyPercent != "" {
		value, err := strconv.ParseFloat(weeklyPercent, 64)
		if err == nil {
			c.state.weeklyUtilization = value
		}
	}
	weeklyResetAt := headers.Get("x-" + activeLimitIdentifier + "-secondary-reset-at")
	if weeklyResetAt != "" {
		value, err := strconv.ParseInt(weeklyResetAt, 10, 64)
		if err == nil {
			c.state.weeklyReset = time.Unix(value, 0)
		}
	}
	c.state.lastUpdated = time.Now()
	if isFirstUpdate || int(c.state.fiveHourUtilization*100) != int(oldFiveHour*100) || int(c.state.weeklyUtilization*100) != int(oldWeekly*100) {
		c.logger.Debug("usage update for ", c.tag, ": 5h=", c.state.fiveHourUtilization, "%, weekly=", c.state.weeklyUtilization, "%")
	}
	shouldInterrupt := c.checkTransitionLocked()
	c.stateMutex.Unlock()
	if shouldInterrupt {
		c.interruptConnections()
	}
}

func (c *externalCredential) checkTransitionLocked() bool {
	unusable := c.state.hardRateLimited || c.state.fiveHourUtilization >= 100 || c.state.weeklyUtilization >= 100
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
		Context:     derived,
		releaseFunc: stop,
		cancelFunc:  cancel,
	}
}

func (c *externalCredential) interruptConnections() {
	c.logger.Warn("interrupting connections for ", c.tag)
	c.requestAccess.Lock()
	c.cancelRequests()
	c.requestContext, c.cancelRequests = context.WithCancel(context.Background())
	c.requestAccess.Unlock()
	if c.onBecameUnusable != nil {
		c.onBecameUnusable()
	}
}

func (c *externalCredential) pollUsage(ctx context.Context) {
	if !c.pollAccess.TryLock() {
		return
	}
	defer c.pollAccess.Unlock()
	defer c.markUsagePollAttempted()

	statusURL := c.baseURL + "/ocm/v1/status"
	httpClient := &http.Client{
		Transport: c.httpClient.Transport,
		Timeout:   5 * time.Second,
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		c.logger.Error("poll usage for ", c.tag, ": create request: ", err)
		return
	}
	request.Header.Set("Authorization", "Bearer "+c.token)

	response, err := httpClient.Do(request)
	if err != nil {
		c.logger.Error("poll usage for ", c.tag, ": ", err)
		c.stateMutex.Lock()
		c.state.consecutivePollFailures++
		c.stateMutex.Unlock()
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		c.stateMutex.Lock()
		c.state.consecutivePollFailures++
		c.stateMutex.Unlock()
		c.logger.Debug("poll usage for ", c.tag, ": status ", response.StatusCode, " ", string(body))
		return
	}

	var statusResponse struct {
		FiveHourUtilization float64 `json:"five_hour_utilization"`
		WeeklyUtilization   float64 `json:"weekly_utilization"`
	}
	err = json.NewDecoder(response.Body).Decode(&statusResponse)
	if err != nil {
		c.stateMutex.Lock()
		c.state.consecutivePollFailures++
		c.stateMutex.Unlock()
		c.logger.Debug("poll usage for ", c.tag, ": decode: ", err)
		return
	}

	c.stateMutex.Lock()
	isFirstUpdate := c.state.lastUpdated.IsZero()
	oldFiveHour := c.state.fiveHourUtilization
	oldWeekly := c.state.weeklyUtilization
	c.state.consecutivePollFailures = 0
	c.state.fiveHourUtilization = statusResponse.FiveHourUtilization
	c.state.weeklyUtilization = statusResponse.WeeklyUtilization
	if c.state.hardRateLimited && time.Now().After(c.state.rateLimitResetAt) {
		c.state.hardRateLimited = false
	}
	if isFirstUpdate || int(c.state.fiveHourUtilization*100) != int(oldFiveHour*100) || int(c.state.weeklyUtilization*100) != int(oldWeekly*100) {
		c.logger.Debug("poll usage for ", c.tag, ": 5h=", c.state.fiveHourUtilization, "%, weekly=", c.state.weeklyUtilization, "%")
	}
	shouldInterrupt := c.checkTransitionLocked()
	c.stateMutex.Unlock()
	if shouldInterrupt {
		c.interruptConnections()
	}
}

func (c *externalCredential) lastUpdatedTime() time.Time {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.state.lastUpdated
}

func (c *externalCredential) markUsagePollAttempted() {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.state.lastUpdated = time.Now()
}

func (c *externalCredential) pollBackoff(baseInterval time.Duration) time.Duration {
	c.stateMutex.RLock()
	failures := c.state.consecutivePollFailures
	c.stateMutex.RUnlock()
	if failures <= 0 {
		return baseInterval
	}
	if failures > 4 {
		failures = 4
	}
	return baseInterval * time.Duration(1<<failures)
}

func (c *externalCredential) usageTrackerOrNil() *AggregatedUsage {
	return c.usageTracker
}

func (c *externalCredential) httpTransport() *http.Client {
	return c.httpClient
}

func (c *externalCredential) ocmDialer() N.Dialer {
	return c.credDialer
}

func (c *externalCredential) ocmIsAPIKeyMode() bool {
	return false
}

func (c *externalCredential) ocmGetAccountID() string {
	return ""
}

func (c *externalCredential) ocmGetBaseURL() string {
	return c.baseURL
}

func (c *externalCredential) close() {
	if c.usageTracker != nil {
		c.usageTracker.cancelPendingSave()
		err := c.usageTracker.Save()
		if err != nil {
			c.logger.Error("save usage statistics for ", c.tag, ": ", err)
		}
	}
}
