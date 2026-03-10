package ccm

import (
	"bytes"
	"context"
	stdTLS "crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/ntp"
)

const defaultPollInterval = 60 * time.Minute

type credentialState struct {
	fiveHourUtilization     float64
	fiveHourReset           time.Time
	weeklyUtilization       float64
	weeklyReset             time.Time
	hardRateLimited         bool
	rateLimitResetAt        time.Time
	accountType             string
	lastUpdated             time.Time
	consecutivePollFailures int
}

type defaultCredential struct {
	tag            string
	credentialPath string
	credentials    *oauthCredentials
	accessMutex    sync.RWMutex
	state          credentialState
	stateMutex     sync.RWMutex
	pollAccess     sync.Mutex
	reserve5h      uint8
	reserveWeekly  uint8
	usageTracker   *AggregatedUsage
	httpClient     *http.Client
	logger         log.ContextLogger

	// Connection interruption
	onBecameUnusable func()
	interrupted      bool
	requestContext   context.Context
	cancelRequests   context.CancelFunc
	requestAccess    sync.Mutex
}

type credentialRequestContext struct {
	context.Context
	releaseOnce sync.Once
	cancelOnce  sync.Once
	releaseFunc func() bool
	cancelFunc  context.CancelFunc
}

func (c *credentialRequestContext) releaseCredentialInterrupt() {
	c.releaseOnce.Do(func() {
		c.releaseFunc()
	})
}

func (c *credentialRequestContext) cancelRequest() {
	c.releaseCredentialInterrupt()
	c.cancelOnce.Do(c.cancelFunc)
}

func newDefaultCredential(ctx context.Context, tag string, options option.CCMDefaultCredentialOptions, logger log.ContextLogger) (*defaultCredential, error) {
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
		reserveWeekly = 10
	}
	requestContext, cancelRequests := context.WithCancel(context.Background())
	credential := &defaultCredential{
		tag:            tag,
		credentialPath: options.CredentialPath,
		reserve5h:      reserve5h,
		reserveWeekly:  reserveWeekly,
		httpClient:     httpClient,
		logger:         logger,
		requestContext: requestContext,
		cancelRequests: cancelRequests,
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
	credentials, err := platformReadCredentials(c.credentialPath)
	if err != nil {
		return E.Cause(err, "read credentials for ", c.tag)
	}
	c.credentials = credentials
	if credentials.SubscriptionType != "" {
		c.state.accountType = credentials.SubscriptionType
	}
	if c.usageTracker != nil {
		err = c.usageTracker.Load()
		if err != nil {
			c.logger.Warn("load usage statistics for ", c.tag, ": ", err)
		}
	}
	return nil
}

func (c *defaultCredential) getAccessToken() (string, error) {
	c.accessMutex.RLock()
	if !c.credentials.needsRefresh() {
		token := c.credentials.AccessToken
		c.accessMutex.RUnlock()
		return token, nil
	}
	c.accessMutex.RUnlock()

	c.accessMutex.Lock()
	defer c.accessMutex.Unlock()

	if !c.credentials.needsRefresh() {
		return c.credentials.AccessToken, nil
	}

	newCredentials, err := refreshToken(c.httpClient, c.credentials)
	if err != nil {
		return "", err
	}

	c.credentials = newCredentials
	if newCredentials.SubscriptionType != "" {
		c.stateMutex.Lock()
		c.state.accountType = newCredentials.SubscriptionType
		c.stateMutex.Unlock()
	}

	err = platformWriteCredentials(newCredentials, c.credentialPath)
	if err != nil {
		c.logger.Warn("persist refreshed token for ", c.tag, ": ", err)
	}

	return newCredentials.AccessToken, nil
}

func parseResetTimestamp(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	unixEpoch, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		return time.Unix(unixEpoch, 0), nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func (c *defaultCredential) updateStateFromHeaders(headers http.Header) {
	c.stateMutex.Lock()
	isFirstUpdate := c.state.lastUpdated.IsZero()
	oldFiveHour := c.state.fiveHourUtilization
	oldWeekly := c.state.weeklyUtilization

	if utilization := headers.Get("anthropic-ratelimit-unified-5h-utilization"); utilization != "" {
		value, err := strconv.ParseFloat(utilization, 64)
		if err == nil {
			newValue := math.Ceil(value * 100)
			if newValue < c.state.fiveHourUtilization {
				c.logger.Error("header 5h utilization for ", c.tag, " is lower than current: ", newValue, " < ", c.state.fiveHourUtilization)
			}
			c.state.fiveHourUtilization = newValue
		}
	}
	if resetAt := headers.Get("anthropic-ratelimit-unified-5h-reset"); resetAt != "" {
		value, err := parseResetTimestamp(resetAt)
		if err == nil {
			c.state.fiveHourReset = value
		}
	}
	if utilization := headers.Get("anthropic-ratelimit-unified-7d-utilization"); utilization != "" {
		value, err := strconv.ParseFloat(utilization, 64)
		if err == nil {
			newValue := math.Ceil(value * 100)
			if newValue < c.state.weeklyUtilization {
				c.logger.Error("header weekly utilization for ", c.tag, " is lower than current: ", newValue, " < ", c.state.weeklyUtilization)
			}
			c.state.weeklyUtilization = newValue
		}
	}
	if resetAt := headers.Get("anthropic-ratelimit-unified-7d-reset"); resetAt != "" {
		value, err := parseResetTimestamp(resetAt)
		if err == nil {
			c.state.weeklyReset = value
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

func (c *defaultCredential) markRateLimited(resetAt time.Time) {
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

func (c *defaultCredential) isUsable() bool {
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
		usable := c.checkReservesLocked()
		c.stateMutex.Unlock()
		return usable
	}
	usable := c.checkReservesLocked()
	c.stateMutex.RUnlock()
	return usable
}

func (c *defaultCredential) checkReservesLocked() bool {
	if c.state.fiveHourUtilization >= float64(100-c.reserve5h) {
		return false
	}
	if c.state.weeklyUtilization >= float64(100-c.reserveWeekly) {
		return false
	}
	return true
}

// checkTransitionLocked detects usable→unusable transition.
// Must be called with stateMutex write lock held.
func (c *defaultCredential) checkTransitionLocked() bool {
	unusable := c.state.hardRateLimited || !c.checkReservesLocked()
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
		Context:     derived,
		releaseFunc: stop,
		cancelFunc:  cancel,
	}
}

func (c *defaultCredential) weeklyUtilization() float64 {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.state.weeklyUtilization
}

func (c *defaultCredential) lastUpdatedTime() time.Time {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.state.lastUpdated
}

func (c *defaultCredential) markUsagePollAttempted() {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.state.lastUpdated = time.Now()
}

func (c *defaultCredential) pollBackoff(baseInterval time.Duration) time.Duration {
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

func (c *defaultCredential) earliestReset() time.Time {
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

const pollUsageMaxRetries = 3

func isTimeoutError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}

func (c *defaultCredential) pollUsage(ctx context.Context) {
	if !c.pollAccess.TryLock() {
		return
	}
	defer c.pollAccess.Unlock()
	defer c.markUsagePollAttempted()

	accessToken, err := c.getAccessToken()
	if err != nil {
		c.logger.Error("poll usage for ", c.tag, ": get token: ", err)
		return
	}

	httpClient := &http.Client{
		Transport: c.httpClient.Transport,
		Timeout:   5 * time.Second,
	}

	var response *http.Response
	for attempt := range pollUsageMaxRetries {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, claudeAPIBaseURL+"/api/oauth/usage", nil)
		if err != nil {
			c.logger.Error("poll usage for ", c.tag, ": create request: ", err)
			return
		}
		request.Header.Set("Authorization", "Bearer "+accessToken)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("User-Agent", ccmUserAgentValue)
		request.Header.Set("anthropic-beta", anthropicBetaOAuthValue)

		response, err = httpClient.Do(request)
		if err == nil {
			break
		}
		if !isTimeoutError(err) {
			c.logger.Error("poll usage for ", c.tag, ": ", err)
			return
		}
		if attempt < pollUsageMaxRetries-1 {
			c.logger.Warn("poll usage for ", c.tag, ": timeout, retrying (", attempt+1, "/", pollUsageMaxRetries, ")")
			continue
		}
		c.logger.Error("poll usage for ", c.tag, ": ", err)
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusTooManyRequests {
			c.logger.Warn("poll usage for ", c.tag, ": rate limited")
		}
		body, _ := io.ReadAll(response.Body)
		c.stateMutex.Lock()
		c.state.consecutivePollFailures++
		c.stateMutex.Unlock()
		c.logger.Debug("poll usage for ", c.tag, ": status ", response.StatusCode, " ", string(body))
		return
	}

	var usageResponse struct {
		FiveHour struct {
			Utilization float64   `json:"utilization"`
			ResetsAt    time.Time `json:"resets_at"`
		} `json:"five_hour"`
		SevenDay struct {
			Utilization float64   `json:"utilization"`
			ResetsAt    time.Time `json:"resets_at"`
		} `json:"seven_day"`
	}
	err = json.NewDecoder(response.Body).Decode(&usageResponse)
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
	c.state.fiveHourUtilization = usageResponse.FiveHour.Utilization
	if !usageResponse.FiveHour.ResetsAt.IsZero() {
		c.state.fiveHourReset = usageResponse.FiveHour.ResetsAt
	}
	c.state.weeklyUtilization = usageResponse.SevenDay.Utilization
	if !usageResponse.SevenDay.ResetsAt.IsZero() {
		c.state.weeklyReset = usageResponse.SevenDay.ResetsAt
	}
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

func (c *defaultCredential) close() {
	if c.usageTracker != nil {
		c.usageTracker.cancelPendingSave()
		err := c.usageTracker.Save()
		if err != nil {
			c.logger.Error("save usage statistics for ", c.tag, ": ", err)
		}
	}
}

// credentialProvider is the interface for all credential types.
type credentialProvider interface {
	selectCredential(sessionID string) (*defaultCredential, bool, error)
	onRateLimited(sessionID string, credential *defaultCredential, resetAt time.Time) *defaultCredential
	pollIfStale(ctx context.Context)
	allDefaults() []*defaultCredential
	close()
}

// singleCredentialProvider wraps a single default credential (legacy or single default).
type singleCredentialProvider struct {
	credential *defaultCredential
}

func (p *singleCredentialProvider) selectCredential(_ string) (*defaultCredential, bool, error) {
	if !p.credential.isUsable() {
		return nil, false, E.New("credential ", p.credential.tag, " is rate-limited")
	}
	return p.credential, false, nil
}

func (p *singleCredentialProvider) onRateLimited(_ string, credential *defaultCredential, resetAt time.Time) *defaultCredential {
	credential.markRateLimited(resetAt)
	return nil
}

func (p *singleCredentialProvider) pollIfStale(ctx context.Context) {
	if time.Since(p.credential.lastUpdatedTime()) > p.credential.pollBackoff(defaultPollInterval) {
		p.credential.pollUsage(ctx)
	}
}

func (p *singleCredentialProvider) allDefaults() []*defaultCredential {
	return []*defaultCredential{p.credential}
}

func (p *singleCredentialProvider) close() {}

const sessionExpiry = 24 * time.Hour

type sessionEntry struct {
	tag       string
	createdAt time.Time
}

// balancerProvider assigns sessions to credentials based on a configurable strategy.
type balancerProvider struct {
	credentials     []*defaultCredential
	strategy        string
	roundRobinIndex atomic.Uint64
	pollInterval    time.Duration
	sessionMutex    sync.RWMutex
	sessions        map[string]sessionEntry
	logger          log.ContextLogger
}

func newBalancerProvider(credentials []*defaultCredential, strategy string, pollInterval time.Duration, logger log.ContextLogger) *balancerProvider {
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	return &balancerProvider{
		credentials:  credentials,
		strategy:     strategy,
		pollInterval: pollInterval,
		sessions:     make(map[string]sessionEntry),
		logger:       logger,
	}
}

func (p *balancerProvider) selectCredential(sessionID string) (*defaultCredential, bool, error) {
	if sessionID != "" {
		p.sessionMutex.RLock()
		entry, exists := p.sessions[sessionID]
		p.sessionMutex.RUnlock()
		if exists {
			for _, credential := range p.credentials {
				if credential.tag == entry.tag && credential.isUsable() {
					return credential, false, nil
				}
			}
			p.sessionMutex.Lock()
			delete(p.sessions, sessionID)
			p.sessionMutex.Unlock()
		}
	}

	best := p.pickCredential()
	if best == nil {
		return nil, false, allCredentialsUnavailableError(p.credentials)
	}

	isNew := sessionID != ""
	if isNew {
		p.sessionMutex.Lock()
		p.sessions[sessionID] = sessionEntry{tag: best.tag, createdAt: time.Now()}
		p.sessionMutex.Unlock()
	}
	return best, isNew, nil
}

func (p *balancerProvider) onRateLimited(sessionID string, credential *defaultCredential, resetAt time.Time) *defaultCredential {
	credential.markRateLimited(resetAt)
	if sessionID != "" {
		p.sessionMutex.Lock()
		delete(p.sessions, sessionID)
		p.sessionMutex.Unlock()
	}

	best := p.pickCredential()
	if best != nil && sessionID != "" {
		p.sessionMutex.Lock()
		p.sessions[sessionID] = sessionEntry{tag: best.tag, createdAt: time.Now()}
		p.sessionMutex.Unlock()
	}
	return best
}

func (p *balancerProvider) pickCredential() *defaultCredential {
	switch p.strategy {
	case "round_robin":
		return p.pickRoundRobin()
	case "random":
		return p.pickRandom()
	default:
		return p.pickLeastUsed()
	}
}

func (p *balancerProvider) pickLeastUsed() *defaultCredential {
	var best *defaultCredential
	bestUtilization := float64(101)
	for _, credential := range p.credentials {
		if !credential.isUsable() {
			continue
		}
		utilization := credential.weeklyUtilization()
		if utilization < bestUtilization {
			bestUtilization = utilization
			best = credential
		}
	}
	return best
}

func (p *balancerProvider) pickRoundRobin() *defaultCredential {
	start := int(p.roundRobinIndex.Add(1) - 1)
	count := len(p.credentials)
	for offset := range count {
		candidate := p.credentials[(start+offset)%count]
		if candidate.isUsable() {
			return candidate
		}
	}
	return nil
}

func (p *balancerProvider) pickRandom() *defaultCredential {
	var usable []*defaultCredential
	for _, candidate := range p.credentials {
		if candidate.isUsable() {
			usable = append(usable, candidate)
		}
	}
	if len(usable) == 0 {
		return nil
	}
	return usable[rand.IntN(len(usable))]
}

func (p *balancerProvider) pollIfStale(ctx context.Context) {
	now := time.Now()
	p.sessionMutex.Lock()
	for id, entry := range p.sessions {
		if now.Sub(entry.createdAt) > sessionExpiry {
			delete(p.sessions, id)
		}
	}
	p.sessionMutex.Unlock()

	for _, credential := range p.credentials {
		if time.Since(credential.lastUpdatedTime()) > credential.pollBackoff(p.pollInterval) {
			credential.pollUsage(ctx)
		}
	}
}

func (p *balancerProvider) allDefaults() []*defaultCredential {
	return p.credentials
}

func (p *balancerProvider) close() {}

// fallbackProvider tries credentials in order.
type fallbackProvider struct {
	credentials  []*defaultCredential
	pollInterval time.Duration
	logger       log.ContextLogger
}

func newFallbackProvider(credentials []*defaultCredential, pollInterval time.Duration, logger log.ContextLogger) *fallbackProvider {
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	return &fallbackProvider{
		credentials:  credentials,
		pollInterval: pollInterval,
		logger:       logger,
	}
}

func (p *fallbackProvider) selectCredential(_ string) (*defaultCredential, bool, error) {
	for _, credential := range p.credentials {
		if credential.isUsable() {
			return credential, false, nil
		}
	}
	return nil, false, allCredentialsUnavailableError(p.credentials)
}

func (p *fallbackProvider) onRateLimited(_ string, credential *defaultCredential, resetAt time.Time) *defaultCredential {
	credential.markRateLimited(resetAt)
	for _, candidate := range p.credentials {
		if candidate.isUsable() {
			return candidate
		}
	}
	return nil
}

func (p *fallbackProvider) pollIfStale(ctx context.Context) {
	for _, credential := range p.credentials {
		if time.Since(credential.lastUpdatedTime()) > credential.pollBackoff(p.pollInterval) {
			credential.pollUsage(ctx)
		}
	}
}

func (p *fallbackProvider) allDefaults() []*defaultCredential {
	return p.credentials
}

func (p *fallbackProvider) close() {}

func allCredentialsUnavailableError(credentials []*defaultCredential) error {
	var earliest time.Time
	for _, credential := range credentials {
		resetAt := credential.earliestReset()
		if !resetAt.IsZero() && (earliest.IsZero() || resetAt.Before(earliest)) {
			earliest = resetAt
		}
	}
	if earliest.IsZero() {
		return E.New("all credentials rate-limited")
	}
	return E.New("all credentials rate-limited, earliest reset in ", log.FormatDuration(time.Until(earliest)))
}

func extractCCMSessionID(bodyBytes []byte) string {
	var body struct {
		Metadata struct {
			UserID string `json:"user_id"`
		} `json:"metadata"`
	}
	err := json.Unmarshal(bodyBytes, &body)
	if err != nil {
		return ""
	}
	userID := body.Metadata.UserID
	sessionIndex := strings.LastIndex(userID, "_session_")
	if sessionIndex < 0 {
		return ""
	}
	return userID[sessionIndex+len("_session_"):]
}

func buildCredentialProviders(
	ctx context.Context,
	options option.CCMServiceOptions,
	logger log.ContextLogger,
) (map[string]credentialProvider, []*defaultCredential, error) {
	defaultCredentials := make(map[string]*defaultCredential)
	var allDefaults []*defaultCredential
	providers := make(map[string]credentialProvider)

	for _, credOpt := range options.Credentials {
		switch credOpt.Type {
		case "default":
			credential, err := newDefaultCredential(ctx, credOpt.Tag, credOpt.DefaultOptions, logger)
			if err != nil {
				return nil, nil, err
			}
			defaultCredentials[credOpt.Tag] = credential
			allDefaults = append(allDefaults, credential)
			providers[credOpt.Tag] = &singleCredentialProvider{credential: credential}
		}
	}

	for _, credOpt := range options.Credentials {
		switch credOpt.Type {
		case "balancer":
			subCredentials, err := resolveCredentialTags(credOpt.BalancerOptions.Credentials, defaultCredentials, credOpt.Tag)
			if err != nil {
				return nil, nil, err
			}
			providers[credOpt.Tag] = newBalancerProvider(subCredentials, credOpt.BalancerOptions.Strategy, time.Duration(credOpt.BalancerOptions.PollInterval), logger)
		case "fallback":
			subCredentials, err := resolveCredentialTags(credOpt.FallbackOptions.Credentials, defaultCredentials, credOpt.Tag)
			if err != nil {
				return nil, nil, err
			}
			providers[credOpt.Tag] = newFallbackProvider(subCredentials, time.Duration(credOpt.FallbackOptions.PollInterval), logger)
		}
	}

	return providers, allDefaults, nil
}

func resolveCredentialTags(tags []string, defaults map[string]*defaultCredential, parentTag string) ([]*defaultCredential, error) {
	credentials := make([]*defaultCredential, 0, len(tags))
	for _, tag := range tags {
		credential, exists := defaults[tag]
		if !exists {
			return nil, E.New("credential ", parentTag, " references unknown default credential: ", tag)
		}
		credentials = append(credentials, credential)
	}
	if len(credentials) == 0 {
		return nil, E.New("credential ", parentTag, " has no sub-credentials")
	}
	return credentials, nil
}

func parseRateLimitResetFromHeaders(headers http.Header) time.Time {
	claim := headers.Get("anthropic-ratelimit-unified-representative-claim")
	switch claim {
	case "5h":
		if resetStr := headers.Get("anthropic-ratelimit-unified-5h-reset"); resetStr != "" {
			value, err := strconv.ParseInt(resetStr, 10, 64)
			if err == nil {
				return time.Unix(value, 0)
			}
		}
	case "7d":
		if resetStr := headers.Get("anthropic-ratelimit-unified-7d-reset"); resetStr != "" {
			value, err := strconv.ParseInt(resetStr, 10, 64)
			if err == nil {
				return time.Unix(value, 0)
			}
		}
	}
	if retryAfter := headers.Get("Retry-After"); retryAfter != "" {
		seconds, err := strconv.ParseInt(retryAfter, 10, 64)
		if err == nil {
			return time.Now().Add(time.Duration(seconds) * time.Second)
		}
	}
	return time.Now().Add(5 * time.Minute)
}

func validateCCMOptions(options option.CCMServiceOptions) error {
	hasCredentials := len(options.Credentials) > 0
	hasLegacyPath := options.CredentialPath != ""
	hasLegacyUsages := options.UsagesPath != ""
	hasLegacyDetour := options.Detour != ""

	if hasCredentials && hasLegacyPath {
		return E.New("credential_path and credentials are mutually exclusive")
	}
	if hasCredentials && hasLegacyUsages {
		return E.New("usages_path and credentials are mutually exclusive; use usages_path on individual credentials")
	}
	if hasCredentials && hasLegacyDetour {
		return E.New("detour and credentials are mutually exclusive; use detour on individual credentials")
	}

	if hasCredentials {
		tags := make(map[string]bool)
		for _, credential := range options.Credentials {
			if tags[credential.Tag] {
				return E.New("duplicate credential tag: ", credential.Tag)
			}
			tags[credential.Tag] = true
			if credential.Type == "default" || credential.Type == "" {
				if credential.DefaultOptions.Reserve5h > 99 {
					return E.New("credential ", credential.Tag, ": reserve_5h must be at most 99")
				}
				if credential.DefaultOptions.ReserveWeekly > 99 {
					return E.New("credential ", credential.Tag, ": reserve_weekly must be at most 99")
				}
			}
			if credential.Type == "balancer" {
				switch credential.BalancerOptions.Strategy {
				case "", "least_used", "round_robin", "random":
				default:
					return E.New("credential ", credential.Tag, ": unknown balancer strategy: ", credential.BalancerOptions.Strategy)
				}
			}
		}

		for _, user := range options.Users {
			if user.Credential == "" {
				return E.New("user ", user.Name, " must specify credential in multi-credential mode")
			}
			if !tags[user.Credential] {
				return E.New("user ", user.Name, " references unknown credential: ", user.Credential)
			}
		}
	}

	return nil
}

// retryRequestWithBody re-sends a buffered request body using a different credential.
func retryRequestWithBody(
	ctx context.Context,
	originalRequest *http.Request,
	bodyBytes []byte,
	credential *defaultCredential,
	httpHeaders http.Header,
) (*http.Response, error) {
	accessToken, err := credential.getAccessToken()
	if err != nil {
		return nil, E.Cause(err, "get access token for ", credential.tag)
	}

	proxyURL := claudeAPIBaseURL + originalRequest.URL.RequestURI()
	retryRequest, err := http.NewRequestWithContext(ctx, originalRequest.Method, proxyURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	for key, values := range originalRequest.Header {
		if !isHopByHopHeader(key) && key != "Authorization" {
			retryRequest.Header[key] = values
		}
	}

	serviceOverridesAcceptEncoding := len(httpHeaders.Values("Accept-Encoding")) > 0
	if credential.usageTracker != nil && !serviceOverridesAcceptEncoding {
		retryRequest.Header.Del("Accept-Encoding")
	}

	anthropicBetaHeader := retryRequest.Header.Get("anthropic-beta")
	if anthropicBetaHeader != "" {
		retryRequest.Header.Set("anthropic-beta", anthropicBetaOAuthValue+","+anthropicBetaHeader)
	} else {
		retryRequest.Header.Set("anthropic-beta", anthropicBetaOAuthValue)
	}

	for key, values := range httpHeaders {
		retryRequest.Header.Del(key)
		retryRequest.Header[key] = values
	}
	retryRequest.Header.Set("Authorization", "Bearer "+accessToken)

	return credential.httpClient.Do(retryRequest)
}

// credentialForUser finds the credential provider for a user.
// In legacy mode, returns the single provider.
// In multi-credential mode, returns the provider mapped to the user's credential tag.
func credentialForUser(
	userCredentialMap map[string]string,
	providers map[string]credentialProvider,
	legacyProvider credentialProvider,
	username string,
) (credentialProvider, error) {
	if legacyProvider != nil {
		return legacyProvider, nil
	}
	tag, exists := userCredentialMap[username]
	if !exists {
		return nil, E.New("no credential mapping for user: ", username)
	}
	provider, exists := providers[tag]
	if !exists {
		return nil, E.New("unknown credential: ", tag)
	}
	return provider, nil
}

// noUserCredentialProvider returns the single provider for legacy mode or the first credential in multi-credential mode (no auth).
func noUserCredentialProvider(
	providers map[string]credentialProvider,
	legacyProvider credentialProvider,
	options option.CCMServiceOptions,
) credentialProvider {
	if legacyProvider != nil {
		return legacyProvider
	}
	if len(options.Credentials) > 0 {
		tag := options.Credentials[0].Tag
		return providers[tag]
	}
	return nil
}
