package ocm

import (
	"bytes"
	"context"
	stdTLS "crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/fswatch"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
)

type defaultCredential struct {
	tag                string
	serviceContext     context.Context
	credentialPath     string
	credentialFilePath string
	credentials        *oauthCredentials
	access             sync.RWMutex
	state              credentialState
	stateAccess        sync.RWMutex
	pollAccess         sync.Mutex
	reloadAccess       sync.Mutex
	watcherAccess      sync.Mutex
	cap5h              float64
	capWeekly          float64
	usageTracker       *AggregatedUsage
	dialer             N.Dialer
	forwardHTTPClient  *http.Client
	logger             log.ContextLogger
	watcher            *fswatch.Watcher
	watcherRetryAt     time.Time

	// Connection interruption
	onBecameUnusable func()
	interrupted      bool
	requestContext   context.Context
	cancelRequests   context.CancelFunc
	requestAccess    sync.Mutex
}

func newDefaultCredential(ctx context.Context, tag string, options option.OCMDefaultCredentialOptions, logger log.ContextLogger) (*defaultCredential, error) {
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
	httpClient := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig: &stdTLS.Config{
				RootCAs: adapter.RootPoolFromContext(ctx),
				Time:    ntp.TimeFuncFromContext(ctx),
			},
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return credentialDialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}
	reserve5h := options.Reserve5h
	if reserve5h == 0 {
		reserve5h = 1
	}
	reserveWeekly := options.ReserveWeekly
	if reserveWeekly == 0 {
		reserveWeekly = 1
	}
	var cap5h float64
	if options.Limit5h > 0 {
		cap5h = float64(options.Limit5h)
	} else {
		cap5h = float64(100 - reserve5h)
	}
	var capWeekly float64
	if options.LimitWeekly > 0 {
		capWeekly = float64(options.LimitWeekly)
	} else {
		capWeekly = float64(100 - reserveWeekly)
	}
	requestContext, cancelRequests := context.WithCancel(context.Background())
	credential := &defaultCredential{
		tag:               tag,
		serviceContext:    ctx,
		credentialPath:    options.CredentialPath,
		cap5h:             cap5h,
		capWeekly:         capWeekly,
		dialer:            credentialDialer,
		forwardHTTPClient: httpClient,
		logger:            logger,
		requestContext:    requestContext,
		cancelRequests:    cancelRequests,
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

func (c *defaultCredential) start() error {
	credentialFilePath, err := resolveCredentialFilePath(c.credentialPath)
	if err != nil {
		return E.Cause(err, "resolve credential path for ", c.tag)
	}
	c.credentialFilePath = credentialFilePath
	err = c.ensureCredentialWatcher()
	if err != nil {
		c.logger.Debug("start credential watcher for ", c.tag, ": ", err)
	}
	err = c.reloadCredentials(true)
	if err != nil {
		c.logger.Warn("initial credential load for ", c.tag, ": ", err)
	}
	if c.usageTracker != nil {
		err = c.usageTracker.Load()
		if err != nil {
			c.logger.Warn("load usage statistics for ", c.tag, ": ", err)
		}
	}
	return nil
}

func (c *defaultCredential) setOnBecameUnusable(fn func()) {
	c.onBecameUnusable = fn
}

func (c *defaultCredential) tagName() string {
	return c.tag
}

func (c *defaultCredential) isExternal() bool {
	return false
}

func (c *defaultCredential) getAccessToken() (string, error) {
	c.retryCredentialReloadIfNeeded()

	c.access.RLock()
	if c.credentials != nil && !c.credentials.needsRefresh() {
		token := c.credentials.getAccessToken()
		c.access.RUnlock()
		return token, nil
	}
	c.access.RUnlock()

	err := c.reloadCredentials(true)
	if err == nil {
		c.access.RLock()
		if c.credentials != nil && !c.credentials.needsRefresh() {
			token := c.credentials.getAccessToken()
			c.access.RUnlock()
			return token, nil
		}
		c.access.RUnlock()
	}

	c.access.Lock()
	defer c.access.Unlock()

	if c.credentials == nil {
		return "", c.unavailableError()
	}
	if !c.credentials.needsRefresh() {
		return c.credentials.getAccessToken(), nil
	}

	err = platformCanWriteCredentials(c.credentialPath)
	if err != nil {
		return "", E.Cause(err, "credential file not writable, refusing refresh to avoid invalidation")
	}

	baseCredentials := cloneCredentials(c.credentials)
	newCredentials, err := refreshToken(c.serviceContext, c.forwardHTTPClient, c.credentials)
	if err != nil {
		return "", err
	}

	latestCredentials, latestErr := platformReadCredentials(c.credentialPath)
	if latestErr == nil && !credentialsEqual(latestCredentials, baseCredentials) {
		c.credentials = latestCredentials
		c.stateAccess.Lock()
		c.state.unavailable = false
		c.state.lastCredentialLoadAttempt = time.Now()
		c.state.lastCredentialLoadError = ""
		c.checkTransitionLocked()
		c.stateAccess.Unlock()
		if !latestCredentials.needsRefresh() {
			return latestCredentials.getAccessToken(), nil
		}
		return "", E.New("credential ", c.tag, " changed while refreshing")
	}

	c.credentials = newCredentials
	c.stateAccess.Lock()
	c.state.unavailable = false
	c.state.lastCredentialLoadAttempt = time.Now()
	c.state.lastCredentialLoadError = ""
	c.checkTransitionLocked()
	c.stateAccess.Unlock()

	err = platformWriteCredentials(newCredentials, c.credentialPath)
	if err != nil {
		c.logger.Error("persist refreshed token for ", c.tag, ": ", err)
	}

	return newCredentials.getAccessToken(), nil
}

func (c *defaultCredential) getAccountID() string {
	c.access.RLock()
	defer c.access.RUnlock()
	if c.credentials == nil {
		return ""
	}
	return c.credentials.getAccountID()
}

func (c *defaultCredential) isAPIKeyMode() bool {
	c.access.RLock()
	defer c.access.RUnlock()
	if c.credentials == nil {
		return false
	}
	return c.credentials.isAPIKeyMode()
}

func (c *defaultCredential) getBaseURL() string {
	if c.isAPIKeyMode() {
		return openaiAPIBaseURL
	}
	return chatGPTBackendURL
}

func (c *defaultCredential) updateStateFromHeaders(headers http.Header) {
	c.stateAccess.Lock()
	isFirstUpdate := c.state.lastUpdated.IsZero()
	oldFiveHour := c.state.fiveHourUtilization
	oldWeekly := c.state.weeklyUtilization
	hadData := false

	activeLimitIdentifier := normalizeRateLimitIdentifier(headers.Get("x-codex-active-limit"))
	if activeLimitIdentifier == "" {
		activeLimitIdentifier = "codex"
	}

	fiveHourResetChanged := false
	fiveHourResetAt := headers.Get("x-" + activeLimitIdentifier + "-primary-reset-at")
	if fiveHourResetAt != "" {
		value, err := strconv.ParseInt(fiveHourResetAt, 10, 64)
		if err == nil {
			hadData = true
			newReset := time.Unix(value, 0)
			if newReset.After(c.state.fiveHourReset) {
				fiveHourResetChanged = true
				c.state.fiveHourReset = newReset
			}
		}
	}
	fiveHourPercent := headers.Get("x-" + activeLimitIdentifier + "-primary-used-percent")
	if fiveHourPercent != "" {
		value, err := strconv.ParseFloat(fiveHourPercent, 64)
		if err == nil {
			hadData = true
			if value >= c.state.fiveHourUtilization || fiveHourResetChanged {
				c.state.fiveHourUtilization = value
			}
		}
	}

	weeklyResetChanged := false
	weeklyResetAt := headers.Get("x-" + activeLimitIdentifier + "-secondary-reset-at")
	if weeklyResetAt != "" {
		value, err := strconv.ParseInt(weeklyResetAt, 10, 64)
		if err == nil {
			hadData = true
			newReset := time.Unix(value, 0)
			if newReset.After(c.state.weeklyReset) {
				weeklyResetChanged = true
				c.state.weeklyReset = newReset
			}
		}
	}
	weeklyPercent := headers.Get("x-" + activeLimitIdentifier + "-secondary-used-percent")
	if weeklyPercent != "" {
		value, err := strconv.ParseFloat(weeklyPercent, 64)
		if err == nil {
			hadData = true
			if value >= c.state.weeklyUtilization || weeklyResetChanged {
				c.state.weeklyUtilization = value
			}
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

func (c *defaultCredential) markRateLimited(resetAt time.Time) {
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

func (c *defaultCredential) isUsable() bool {
	c.retryCredentialReloadIfNeeded()

	c.stateAccess.RLock()
	if c.state.unavailable {
		c.stateAccess.RUnlock()
		return false
	}
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
		usable := c.checkReservesLocked()
		c.stateAccess.Unlock()
		return usable
	}
	usable := c.checkReservesLocked()
	c.stateAccess.RUnlock()
	return usable
}

func (c *defaultCredential) checkReservesLocked() bool {
	if c.state.fiveHourUtilization >= c.cap5h {
		return false
	}
	if c.state.weeklyUtilization >= c.capWeekly {
		return false
	}
	return true
}

// checkTransitionLocked detects usable->unusable transition.
// Must be called with stateAccess write lock held.
func (c *defaultCredential) checkTransitionLocked() bool {
	unusable := c.state.unavailable || c.state.hardRateLimited || !c.checkReservesLocked() || c.state.consecutivePollFailures > 0
	if unusable && !c.interrupted {
		c.interrupted = true
		return true
	}
	if !unusable && c.interrupted {
		c.interrupted = false
	}
	return false
}

func (c *defaultCredential) interruptConnections() {
	c.logger.Warn("interrupting connections for ", c.tag)
	c.requestAccess.Lock()
	c.cancelRequests()
	c.requestContext, c.cancelRequests = context.WithCancel(context.Background())
	c.requestAccess.Unlock()
	if c.onBecameUnusable != nil {
		c.onBecameUnusable()
	}
}

func (c *defaultCredential) wrapRequestContext(parent context.Context) *credentialRequestContext {
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

func (c *defaultCredential) fiveHourUtilization() float64 {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	return c.state.fiveHourUtilization
}

func (c *defaultCredential) weeklyUtilization() float64 {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	return c.state.weeklyUtilization
}

func (c *defaultCredential) planWeight() float64 {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	return ocmPlanWeight(c.state.accountType)
}

func (c *defaultCredential) weeklyResetTime() time.Time {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	return c.state.weeklyReset
}

func (c *defaultCredential) isAvailable() bool {
	c.retryCredentialReloadIfNeeded()

	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	return !c.state.unavailable
}

func (c *defaultCredential) unavailableError() error {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	if !c.state.unavailable {
		return nil
	}
	if c.state.lastCredentialLoadError == "" {
		return E.New("credential ", c.tag, " is unavailable")
	}
	return E.New("credential ", c.tag, " is unavailable: ", c.state.lastCredentialLoadError)
}

func (c *defaultCredential) lastUpdatedTime() time.Time {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	return c.state.lastUpdated
}

func (c *defaultCredential) markUsagePollAttempted() {
	c.stateAccess.Lock()
	defer c.stateAccess.Unlock()
	c.state.lastUpdated = time.Now()
}

func (c *defaultCredential) incrementPollFailures() {
	c.stateAccess.Lock()
	c.state.consecutivePollFailures++
	shouldInterrupt := c.checkTransitionLocked()
	c.stateAccess.Unlock()
	if shouldInterrupt {
		c.interruptConnections()
	}
}

func (c *defaultCredential) pollBackoff(baseInterval time.Duration) time.Duration {
	c.stateAccess.RLock()
	failures := c.state.consecutivePollFailures
	c.stateAccess.RUnlock()
	if failures <= 0 {
		return baseInterval
	}
	backoff := failedPollRetryInterval * time.Duration(1<<(failures-1))
	if backoff > httpRetryMaxBackoff {
		return httpRetryMaxBackoff
	}
	return backoff
}

func (c *defaultCredential) isPollBackoffAtCap() bool {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	failures := c.state.consecutivePollFailures
	return failures > 0 && failedPollRetryInterval*time.Duration(1<<(failures-1)) >= httpRetryMaxBackoff
}

func (c *defaultCredential) earliestReset() time.Time {
	c.stateAccess.RLock()
	defer c.stateAccess.RUnlock()
	if c.state.unavailable {
		return time.Time{}
	}
	if c.state.hardRateLimited {
		return c.state.rateLimitResetAt
	}
	earliest := c.state.fiveHourReset
	if !c.state.weeklyReset.IsZero() && (earliest.IsZero() || c.state.weeklyReset.Before(earliest)) {
		earliest = c.state.weeklyReset
	}
	return earliest
}

func (c *defaultCredential) fiveHourCap() float64 {
	return c.cap5h
}

func (c *defaultCredential) weeklyCap() float64 {
	return c.capWeekly
}

func (c *defaultCredential) usageTrackerOrNil() *AggregatedUsage {
	return c.usageTracker
}

func (c *defaultCredential) httpClient() *http.Client {
	return c.forwardHTTPClient
}

func (c *defaultCredential) ocmDialer() N.Dialer {
	return c.dialer
}

func (c *defaultCredential) ocmIsAPIKeyMode() bool {
	return c.isAPIKeyMode()
}

func (c *defaultCredential) ocmGetAccountID() string {
	return c.getAccountID()
}

func (c *defaultCredential) ocmGetBaseURL() string {
	return c.getBaseURL()
}

func (c *defaultCredential) pollUsage(ctx context.Context) {
	if !c.pollAccess.TryLock() {
		return
	}
	defer c.pollAccess.Unlock()
	defer c.markUsagePollAttempted()

	c.retryCredentialReloadIfNeeded()
	if !c.isAvailable() {
		return
	}
	if c.isAPIKeyMode() {
		return
	}

	accessToken, err := c.getAccessToken()
	if err != nil {
		if !c.isPollBackoffAtCap() {
			c.logger.Error("poll usage for ", c.tag, ": get token: ", err)
		}
		c.incrementPollFailures()
		return
	}

	var usageURL string
	if c.isAPIKeyMode() {
		usageURL = openaiAPIBaseURL + "/api/codex/usage"
	} else {
		usageURL = strings.TrimSuffix(chatGPTBackendURL, "/codex") + "/wham/usage"
	}

	accountID := c.getAccountID()
	pollClient := &http.Client{
		Transport: c.forwardHTTPClient.Transport,
		Timeout:   5 * time.Second,
	}

	response, err := doHTTPWithRetry(ctx, pollClient, func() (*http.Request, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, usageURL, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Authorization", "Bearer "+accessToken)
		if accountID != "" {
			request.Header.Set("ChatGPT-Account-Id", accountID)
		}
		return request, nil
	})
	if err != nil {
		if !c.isPollBackoffAtCap() {
			c.logger.Error("poll usage for ", c.tag, ": ", err)
		}
		c.incrementPollFailures()
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusTooManyRequests {
			c.logger.Warn("poll usage for ", c.tag, ": rate limited")
		}
		body, _ := io.ReadAll(response.Body)
		c.logger.Debug("poll usage for ", c.tag, ": status ", response.StatusCode, " ", string(body))
		c.incrementPollFailures()
		return
	}

	type usageWindow struct {
		UsedPercent float64 `json:"used_percent"`
		ResetAt     int64   `json:"reset_at"`
	}
	var usageResponse struct {
		PlanType  string `json:"plan_type"`
		RateLimit *struct {
			PrimaryWindow   *usageWindow `json:"primary_window"`
			SecondaryWindow *usageWindow `json:"secondary_window"`
		} `json:"rate_limit"`
	}
	err = json.NewDecoder(response.Body).Decode(&usageResponse)
	if err != nil {
		c.logger.Debug("poll usage for ", c.tag, ": decode: ", err)
		c.incrementPollFailures()
		return
	}

	c.stateAccess.Lock()
	isFirstUpdate := c.state.lastUpdated.IsZero()
	oldFiveHour := c.state.fiveHourUtilization
	oldWeekly := c.state.weeklyUtilization
	c.state.consecutivePollFailures = 0
	if usageResponse.RateLimit != nil {
		if w := usageResponse.RateLimit.PrimaryWindow; w != nil {
			c.state.fiveHourUtilization = w.UsedPercent
			if w.ResetAt > 0 {
				c.state.fiveHourReset = time.Unix(w.ResetAt, 0)
			}
		}
		if w := usageResponse.RateLimit.SecondaryWindow; w != nil {
			c.state.weeklyUtilization = w.UsedPercent
			if w.ResetAt > 0 {
				c.state.weeklyReset = time.Unix(w.ResetAt, 0)
			}
		}
	}
	if usageResponse.PlanType != "" {
		c.state.accountType = usageResponse.PlanType
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

func (c *defaultCredential) buildProxyRequest(ctx context.Context, original *http.Request, bodyBytes []byte, serviceHeaders http.Header) (*http.Request, error) {
	accessToken, err := c.getAccessToken()
	if err != nil {
		return nil, E.Cause(err, "get access token for ", c.tag)
	}

	path := original.URL.Path
	var proxyPath string
	if c.isAPIKeyMode() {
		proxyPath = path
	} else {
		proxyPath = strings.TrimPrefix(path, "/v1")
	}

	proxyURL := c.getBaseURL() + proxyPath
	if original.URL.RawQuery != "" {
		proxyURL += "?" + original.URL.RawQuery
	}

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
		if !isHopByHopHeader(key) && !isReverseProxyHeader(key) && key != "Authorization" {
			proxyRequest.Header[key] = values
		}
	}

	for key, values := range serviceHeaders {
		proxyRequest.Header.Del(key)
		proxyRequest.Header[key] = values
	}
	proxyRequest.Header.Set("Authorization", "Bearer "+accessToken)

	if accountID := c.getAccountID(); accountID != "" {
		proxyRequest.Header.Set("ChatGPT-Account-Id", accountID)
	}

	return proxyRequest, nil
}

func (c *defaultCredential) close() {
	if c.watcher != nil {
		err := c.watcher.Close()
		if err != nil {
			c.logger.Error("close credential watcher for ", c.tag, ": ", err)
		}
	}
	if c.usageTracker != nil {
		c.usageTracker.cancelPendingSave()
		err := c.usageTracker.Save()
		if err != nil {
			c.logger.Error("save usage statistics for ", c.tag, ": ", err)
		}
	}
}
