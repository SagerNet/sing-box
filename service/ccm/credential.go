package ccm

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	defaultPollInterval     = 60 * time.Minute
	failedPollRetryInterval = time.Minute
	httpRetryMaxBackoff     = 5 * time.Minute
)

const (
	httpRetryMaxAttempts  = 3
	httpRetryInitialDelay = 200 * time.Millisecond
)

const sessionExpiry = 24 * time.Hour

func doHTTPWithRetry(ctx context.Context, client *http.Client, buildRequest func() (*http.Request, error)) (*http.Response, error) {
	var lastError error
	for attempt := range httpRetryMaxAttempts {
		if attempt > 0 {
			delay := httpRetryInitialDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, lastError
			case <-time.After(delay):
			}
		}
		request, err := buildRequest()
		if err != nil {
			return nil, err
		}
		response, err := client.Do(request)
		if err == nil {
			return response, nil
		}
		lastError = err
		if ctx.Err() != nil {
			return nil, lastError
		}
	}
	return nil, lastError
}

type credentialState struct {
	fiveHourUtilization       float64
	fiveHourReset             time.Time
	weeklyUtilization         float64
	weeklyReset               time.Time
	hardRateLimited           bool
	rateLimitResetAt          time.Time
	accountType               string
	rateLimitTier             string
	remotePlanWeight          float64
	lastUpdated               time.Time
	consecutivePollFailures   int
	unavailable               bool
	lastCredentialLoadAttempt time.Time
	lastCredentialLoadError   string
}

type credentialRequestContext struct {
	context.Context
	releaseOnce  sync.Once
	cancelOnce   sync.Once
	releaseFuncs []func() bool
	cancelFunc   context.CancelFunc
}

func (c *credentialRequestContext) addInterruptLink(stop func() bool) {
	c.releaseFuncs = append(c.releaseFuncs, stop)
}

func (c *credentialRequestContext) releaseCredentialInterrupt() {
	c.releaseOnce.Do(func() {
		for _, f := range c.releaseFuncs {
			f()
		}
	})
}

func (c *credentialRequestContext) cancelRequest() {
	c.releaseCredentialInterrupt()
	c.cancelOnce.Do(c.cancelFunc)
}

type credential interface {
	tagName() string
	isAvailable() bool
	isUsable() bool
	isExternal() bool
	fiveHourUtilization() float64
	weeklyUtilization() float64
	fiveHourCap() float64
	weeklyCap() float64
	planWeight() float64
	weeklyResetTime() time.Time
	markRateLimited(resetAt time.Time)
	earliestReset() time.Time
	unavailableError() error

	getAccessToken() (string, error)
	buildProxyRequest(ctx context.Context, original *http.Request, bodyBytes []byte, serviceHeaders http.Header) (*http.Request, error)
	updateStateFromHeaders(header http.Header)

	wrapRequestContext(ctx context.Context) *credentialRequestContext
	interruptConnections()

	start() error
	pollUsage(ctx context.Context)
	lastUpdatedTime() time.Time
	pollBackoff(base time.Duration) time.Duration
	usageTrackerOrNil() *AggregatedUsage
	httpClient() *http.Client
	close()
}

type credentialSelectionScope string

const (
	credentialSelectionScopeAll         credentialSelectionScope = "all"
	credentialSelectionScopeNonExternal credentialSelectionScope = "non_external"
)

type credentialSelection struct {
	scope  credentialSelectionScope
	filter func(credential) bool
}

func (s credentialSelection) allows(cred credential) bool {
	return s.filter == nil || s.filter(cred)
}

func (s credentialSelection) scopeOrDefault() credentialSelectionScope {
	if s.scope == "" {
		return credentialSelectionScopeAll
	}
	return s.scope
}

// Claude Code's unified rate-limit handling parses these reset headers with
// Number(...), compares them against Date.now()/1000, and renders them via
// new Date(seconds*1000), so keep the wire format pinned to Unix epoch seconds.
func parseAnthropicResetHeaderValue(headerName string, headerValue string) time.Time {
	unixEpoch, err := strconv.ParseInt(headerValue, 10, 64)
	if err != nil {
		panic("invalid " + headerName + " header: expected Unix epoch seconds, got " + strconv.Quote(headerValue))
	}
	if unixEpoch <= 0 {
		panic("invalid " + headerName + " header: expected positive Unix epoch seconds, got " + strconv.Quote(headerValue))
	}
	return time.Unix(unixEpoch, 0)
}

func parseOptionalAnthropicResetHeader(headers http.Header, headerName string) (time.Time, bool) {
	headerValue := headers.Get(headerName)
	if headerValue == "" {
		return time.Time{}, false
	}
	return parseAnthropicResetHeaderValue(headerName, headerValue), true
}

func parseRequiredAnthropicResetHeader(headers http.Header, headerName string) time.Time {
	headerValue := headers.Get(headerName)
	if headerValue == "" {
		panic("missing required " + headerName + " header")
	}
	return parseAnthropicResetHeaderValue(headerName, headerValue)
}

func parseRateLimitResetFromHeaders(headers http.Header) time.Time {
	claim := headers.Get("anthropic-ratelimit-unified-representative-claim")
	switch claim {
	case "5h":
		return parseRequiredAnthropicResetHeader(headers, "anthropic-ratelimit-unified-5h-reset")
	case "7d":
		return parseRequiredAnthropicResetHeader(headers, "anthropic-ratelimit-unified-7d-reset")
	default:
		panic("invalid anthropic-ratelimit-unified-representative-claim header: " + strconv.Quote(claim))
	}
}
