package ocm

import (
	"context"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

type credentialProvider interface {
	selectCredential(sessionID string, selection credentialSelection) (credential, bool, error)
	onRateLimited(sessionID string, cred credential, resetAt time.Time, selection credentialSelection) credential
	linkProviderInterrupt(cred credential, selection credentialSelection, onInterrupt func()) func() bool
	pollIfStale(ctx context.Context)
	allCredentials() []credential
	close()
}

type singleCredentialProvider struct {
	cred          credential
	sessionAccess sync.RWMutex
	sessions      map[string]time.Time
}

func (p *singleCredentialProvider) selectCredential(sessionID string, selection credentialSelection) (credential, bool, error) {
	if !selection.allows(p.cred) {
		return nil, false, E.New("credential ", p.cred.tagName(), " is filtered out")
	}
	if !p.cred.isAvailable() {
		return nil, false, p.cred.unavailableError()
	}
	if !p.cred.isUsable() {
		return nil, false, E.New("credential ", p.cred.tagName(), " is rate-limited")
	}
	var isNew bool
	if sessionID != "" {
		p.sessionAccess.Lock()
		if p.sessions == nil {
			p.sessions = make(map[string]time.Time)
		}
		_, exists := p.sessions[sessionID]
		if !exists {
			p.sessions[sessionID] = time.Now()
			isNew = true
		}
		p.sessionAccess.Unlock()
	}
	return p.cred, isNew, nil
}

func (p *singleCredentialProvider) onRateLimited(_ string, cred credential, resetAt time.Time, _ credentialSelection) credential {
	cred.markRateLimited(resetAt)
	return nil
}

func (p *singleCredentialProvider) pollIfStale(ctx context.Context) {
	now := time.Now()
	p.sessionAccess.Lock()
	for id, createdAt := range p.sessions {
		if now.Sub(createdAt) > sessionExpiry {
			delete(p.sessions, id)
		}
	}
	p.sessionAccess.Unlock()

	if time.Since(p.cred.lastUpdatedTime()) > p.cred.pollBackoff(defaultPollInterval) {
		p.cred.pollUsage(ctx)
	}
}

func (p *singleCredentialProvider) allCredentials() []credential {
	return []credential{p.cred}
}

func (p *singleCredentialProvider) linkProviderInterrupt(_ credential, _ credentialSelection, _ func()) func() bool {
	return func() bool {
		return false
	}
}

func (p *singleCredentialProvider) close() {}

type sessionEntry struct {
	tag            string
	selectionScope credentialSelectionScope
	createdAt      time.Time
}

type credentialInterruptKey struct {
	tag            string
	selectionScope credentialSelectionScope
}

type credentialInterruptEntry struct {
	context context.Context
	cancel  context.CancelFunc
}

type balancerProvider struct {
	credentials          []credential
	strategy             string
	roundRobinIndex      atomic.Uint64
	pollInterval         time.Duration
	rebalanceThreshold   float64
	sessionAccess        sync.RWMutex
	sessions             map[string]sessionEntry
	interruptAccess      sync.Mutex
	credentialInterrupts map[credentialInterruptKey]credentialInterruptEntry
	logger               log.ContextLogger
}

func compositeCredentialSelectable(cred credential) bool {
	return !cred.ocmIsAPIKeyMode()
}

func newBalancerProvider(credentials []credential, strategy string, pollInterval time.Duration, rebalanceThreshold float64, logger log.ContextLogger) *balancerProvider {
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	return &balancerProvider{
		credentials:          credentials,
		strategy:             strategy,
		pollInterval:         pollInterval,
		rebalanceThreshold:   rebalanceThreshold,
		sessions:             make(map[string]sessionEntry),
		credentialInterrupts: make(map[credentialInterruptKey]credentialInterruptEntry),
		logger:               logger,
	}
}

func (p *balancerProvider) selectCredential(sessionID string, selection credentialSelection) (credential, bool, error) {
	if p.strategy == C.BalancerStrategyFallback {
		best := p.pickCredential(selection.filter)
		if best == nil {
			return nil, false, allRateLimitedError(p.credentials)
		}
		return best, false, nil
	}

	selectionScope := selection.scopeOrDefault()
	if sessionID != "" {
		p.sessionAccess.RLock()
		entry, exists := p.sessions[sessionID]
		p.sessionAccess.RUnlock()
		if exists {
			if entry.selectionScope == selectionScope {
				for _, cred := range p.credentials {
					if cred.tagName() == entry.tag && compositeCredentialSelectable(cred) && selection.allows(cred) && cred.isUsable() {
						if p.rebalanceThreshold > 0 && (p.strategy == "" || p.strategy == C.BalancerStrategyLeastUsed) {
							better := p.pickLeastUsed(selection.filter)
							if better != nil && better.tagName() != cred.tagName() {
								effectiveThreshold := p.rebalanceThreshold / cred.planWeight()
								delta := cred.weeklyUtilization() - better.weeklyUtilization()
								if delta > effectiveThreshold {
									p.logger.Info("rebalancing away from ", cred.tagName(),
										": utilization delta ", delta, "% exceeds effective threshold ",
										effectiveThreshold, "% (weight ", cred.planWeight(), ")")
									p.rebalanceCredential(cred.tagName(), selectionScope)
									break
								}
							}
						}
						return cred, false, nil
					}
				}
			}
			p.sessionAccess.Lock()
			delete(p.sessions, sessionID)
			p.sessionAccess.Unlock()
		}
	}

	best := p.pickCredential(selection.filter)
	if best == nil {
		return nil, false, allRateLimitedError(p.credentials)
	}

	isNew := sessionID != ""
	if isNew {
		p.sessionAccess.Lock()
		p.sessions[sessionID] = sessionEntry{
			tag:            best.tagName(),
			selectionScope: selectionScope,
			createdAt:      time.Now(),
		}
		p.sessionAccess.Unlock()
	}
	return best, isNew, nil
}

func (p *balancerProvider) rebalanceCredential(tag string, selectionScope credentialSelectionScope) {
	key := credentialInterruptKey{tag: tag, selectionScope: selectionScope}
	p.interruptAccess.Lock()
	if entry, loaded := p.credentialInterrupts[key]; loaded {
		entry.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.credentialInterrupts[key] = credentialInterruptEntry{context: ctx, cancel: cancel}
	p.interruptAccess.Unlock()

	p.sessionAccess.Lock()
	for id, entry := range p.sessions {
		if entry.tag == tag && entry.selectionScope == selectionScope {
			delete(p.sessions, id)
		}
	}
	p.sessionAccess.Unlock()
}

func (p *balancerProvider) linkProviderInterrupt(cred credential, selection credentialSelection, onInterrupt func()) func() bool {
	if p.strategy == C.BalancerStrategyFallback {
		return func() bool { return false }
	}
	key := credentialInterruptKey{
		tag:            cred.tagName(),
		selectionScope: selection.scopeOrDefault(),
	}
	p.interruptAccess.Lock()
	entry, loaded := p.credentialInterrupts[key]
	if !loaded {
		ctx, cancel := context.WithCancel(context.Background())
		entry = credentialInterruptEntry{context: ctx, cancel: cancel}
		p.credentialInterrupts[key] = entry
	}
	p.interruptAccess.Unlock()
	return context.AfterFunc(entry.context, onInterrupt)
}

func (p *balancerProvider) onRateLimited(sessionID string, cred credential, resetAt time.Time, selection credentialSelection) credential {
	cred.markRateLimited(resetAt)
	if p.strategy == C.BalancerStrategyFallback {
		return p.pickCredential(selection.filter)
	}
	if sessionID != "" {
		p.sessionAccess.Lock()
		delete(p.sessions, sessionID)
		p.sessionAccess.Unlock()
	}

	best := p.pickCredential(selection.filter)
	if best != nil && sessionID != "" {
		p.sessionAccess.Lock()
		p.sessions[sessionID] = sessionEntry{
			tag:            best.tagName(),
			selectionScope: selection.scopeOrDefault(),
			createdAt:      time.Now(),
		}
		p.sessionAccess.Unlock()
	}
	return best
}

func (p *balancerProvider) pickCredential(filter func(credential) bool) credential {
	switch p.strategy {
	case C.BalancerStrategyRoundRobin:
		return p.pickRoundRobin(filter)
	case C.BalancerStrategyRandom:
		return p.pickRandom(filter)
	case C.BalancerStrategyFallback:
		return p.pickFallback(filter)
	default:
		return p.pickLeastUsed(filter)
	}
}

func (p *balancerProvider) pickFallback(filter func(credential) bool) credential {
	for _, cred := range p.credentials {
		if filter != nil && !filter(cred) {
			continue
		}
		if !compositeCredentialSelectable(cred) {
			continue
		}
		if cred.isUsable() {
			return cred
		}
	}
	return nil
}

const weeklyWindowHours = 7 * 24

func (p *balancerProvider) pickLeastUsed(filter func(credential) bool) credential {
	var best credential
	bestScore := float64(-1)
	now := time.Now()
	for _, cred := range p.credentials {
		if filter != nil && !filter(cred) {
			continue
		}
		if !compositeCredentialSelectable(cred) {
			continue
		}
		if !cred.isUsable() {
			continue
		}
		remaining := cred.weeklyCap() - cred.weeklyUtilization()
		score := remaining * cred.planWeight()
		resetTime := cred.weeklyResetTime()
		if !resetTime.IsZero() {
			timeUntilReset := resetTime.Sub(now)
			if timeUntilReset < time.Hour {
				timeUntilReset = time.Hour
			}
			score *= weeklyWindowHours / timeUntilReset.Hours()
		}
		if score > bestScore {
			bestScore = score
			best = cred
		}
	}
	return best
}

func ocmPlanWeight(accountType string) float64 {
	switch accountType {
	case "pro":
		return 10
	case "plus":
		return 1
	default:
		return 1
	}
}

func (p *balancerProvider) pickRoundRobin(filter func(credential) bool) credential {
	start := int(p.roundRobinIndex.Add(1) - 1)
	count := len(p.credentials)
	for offset := range count {
		candidate := p.credentials[(start+offset)%count]
		if filter != nil && !filter(candidate) {
			continue
		}
		if !compositeCredentialSelectable(candidate) {
			continue
		}
		if candidate.isUsable() {
			return candidate
		}
	}
	return nil
}

func (p *balancerProvider) pickRandom(filter func(credential) bool) credential {
	var usable []credential
	for _, candidate := range p.credentials {
		if filter != nil && !filter(candidate) {
			continue
		}
		if !compositeCredentialSelectable(candidate) {
			continue
		}
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
	p.sessionAccess.Lock()
	for id, entry := range p.sessions {
		if now.Sub(entry.createdAt) > sessionExpiry {
			delete(p.sessions, id)
		}
	}
	p.sessionAccess.Unlock()

	for _, cred := range p.credentials {
		if time.Since(cred.lastUpdatedTime()) > cred.pollBackoff(p.pollInterval) {
			cred.pollUsage(ctx)
		}
	}
}

func (p *balancerProvider) allCredentials() []credential {
	return p.credentials
}

func (p *balancerProvider) close() {}

func allRateLimitedError(credentials []credential) error {
	var hasUnavailable bool
	var earliest time.Time
	for _, cred := range credentials {
		if cred.unavailableError() != nil {
			hasUnavailable = true
			continue
		}
		resetAt := cred.earliestReset()
		if !resetAt.IsZero() && (earliest.IsZero() || resetAt.Before(earliest)) {
			earliest = resetAt
		}
	}
	if hasUnavailable {
		return E.New("all credentials unavailable")
	}
	if earliest.IsZero() {
		return E.New("all credentials rate-limited")
	}
	return E.New("all credentials rate-limited, earliest reset in ", log.FormatDuration(time.Until(earliest)))
}
