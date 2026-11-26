//go:build with_tailscale

package tailssh

import (
	"context"
	"errors"
	"maps"
	"net"
	"net/netip"
	"os/user"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/tailscale/tailcfg"
)

const (
	loginGraceTime       = 120 * time.Second
	delegateFetchTimeout = 30 * time.Minute
	holdDelegateGrace    = delegateFetchTimeout + 5*time.Minute
	shutdownTimeout      = 10 * time.Second
	ioDrainTimeout       = 30 * time.Second
	revokeFlushTimeout   = 3 * time.Second
)

var errUserMismatch = E.New("tailnet policy does not permit the requested user")

type terminationError struct {
	message string
}

func (e *terminationError) Error() string {
	return e.message
}

type userLookupFunc func(username string) (*adapter.PlatformUser, error)

type sshConn struct {
	server     *Server
	netConn    net.Conn
	authCtx    context.Context
	authCancel context.CancelFunc

	node            tailcfg.NodeView
	userProfile     tailcfg.UserProfile
	sshUser         string
	srcIP           netip.Addr
	connID          string
	action0         *tailcfg.SSHAction
	action          *tailcfg.SSHAction
	latchedRule     *tailcfg.SSHRule
	mappedLocalUser string
	localUser       *adapter.PlatformUser
	acceptEnv       []string

	authCompleted atomic.Bool
	recheckAccess sync.Mutex
	access        sync.Mutex
	sessions      map[*activeSession]struct{}
	graceTimer    *time.Timer
	ruleTimer     *time.Timer
	teardown      bool
	closeOnce     sync.Once
}

type activeSession struct {
	cancel context.CancelCauseFunc
	done   chan struct{}
}

type policyMatch struct {
	action    *tailcfg.SSHAction
	rule      *tailcfg.SSHRule
	localUser string
	acceptEnv []string
}

func (c *sshConn) evaluatePolicy(policy *tailcfg.SSHPolicy, lookup userLookupFunc) (*policyMatch, error) {
	userMismatch := false
	for _, rule := range policy.Rules {
		if rule == nil || rule.Action == nil || ruleExpired(rule) {
			continue
		}
		if !c.matchPrincipals(rule.Principals) {
			continue
		}
		if rule.Action.Reject {
			return &policyMatch{action: rule.Action, rule: rule}, nil
		}
		localUser := mapLocalUser(rule.SSHUsers, c.sshUser, lookup)
		if localUser == "" {
			userMismatch = true
			continue
		}
		return &policyMatch{
			action:    rule.Action,
			rule:      rule,
			localUser: localUser,
			acceptEnv: rule.AcceptEnv,
		}, nil
	}
	if userMismatch {
		return nil, errUserMismatch
	}
	return nil, E.New("no matching SSH rule")
}

func (c *sshConn) matchPrincipals(principals []*tailcfg.SSHPrincipal) bool {
	for _, principal := range principals {
		if principal == nil {
			continue
		}
		if principal.Any {
			return true
		}
		if !principal.Node.IsZero() && c.node.Valid() && principal.Node == c.node.StableID() {
			return true
		}
		if principal.NodeIP != "" {
			principalIP, err := netip.ParseAddr(principal.NodeIP)
			if err == nil && principalIP == c.srcIP {
				return true
			}
		}
		if principal.UserLogin != "" && principal.UserLogin == c.userProfile.LoginName {
			return true
		}
	}
	return false
}

func mapLocalUser(sshUsers map[string]string, requestedUser string, lookup userLookupFunc) string {
	localUser, loaded := sshUsers[requestedUser]
	if !loaded {
		localUser = sshUsers["*"]
	}
	if localUser != "=" {
		return localUser
	}
	user, err := lookup(requestedUser)
	if err != nil || isRootUser(user) {
		return ""
	}
	return requestedUser
}

func isRootUser(user *adapter.PlatformUser) bool {
	return user.Username == "root" || user.Uid == 0
}

func ruleExpired(rule *tailcfg.SSHRule) bool {
	return rule.RuleExpires != nil && rule.RuleExpires.Before(time.Now())
}

func isUserMissing(err error) bool {
	_, nameMissing := errors.AsType[user.UnknownUserError](err)
	_, uidMissing := errors.AsType[user.UnknownUserIdError](err)
	return nameMissing || uidMissing
}

func (c *sshConn) armLoginGraceLocked(duration time.Duration) {
	if c.graceTimer != nil {
		c.graceTimer.Stop()
	}
	var timer *time.Timer
	timer = time.AfterFunc(duration, func() {
		c.access.Lock()
		if c.authCompleted.Load() || c.graceTimer != timer {
			c.access.Unlock()
			return
		}
		c.graceTimer = nil
		c.access.Unlock()
		c.server.logger.Info("SSH login grace period expired before authentication completed")
		c.close()
	})
	c.graceTimer = timer
}

func (c *sshConn) extendLoginGrace(duration time.Duration) {
	c.access.Lock()
	defer c.access.Unlock()
	c.armLoginGraceLocked(duration)
}

func (c *sshConn) completeAuth() {
	c.access.Lock()
	c.authCompleted.Store(true)
	if c.graceTimer != nil {
		c.graceTimer.Stop()
		c.graceTimer = nil
	}
	c.access.Unlock()
	go c.recheckPolicy(c.server.lookupUser)
}

func (c *sshConn) armRuleExpiryLocked(rule *tailcfg.SSHRule) {
	if c.ruleTimer != nil {
		c.ruleTimer.Stop()
		c.ruleTimer = nil
	}
	if rule == nil || rule.RuleExpires == nil {
		return
	}
	duration := max(time.Until(*rule.RuleExpires), time.Millisecond)
	c.ruleTimer = time.AfterFunc(duration, func() {
		c.recheckPolicy(c.server.lookupUser)
	})
}

func (c *sshConn) stopTimers() {
	c.access.Lock()
	defer c.access.Unlock()
	c.teardown = true
	if c.graceTimer != nil {
		c.graceTimer.Stop()
		c.graceTimer = nil
	}
	if c.ruleTimer != nil {
		c.ruleTimer.Stop()
		c.ruleTimer = nil
	}
}

func (c *sshConn) close() {
	c.closeOnce.Do(func() {
		c.authCancel()
		c.netConn.Close()
	})
}

func (c *sshConn) attachSession(session *activeSession) bool {
	c.server.access.Lock()
	defer c.server.access.Unlock()
	if c.server.closed {
		return false
	}
	c.server.sessionWg.Add(1)
	c.access.Lock()
	c.sessions[session] = struct{}{}
	c.access.Unlock()
	return true
}

func (c *sshConn) detachSession(session *activeSession) {
	c.access.Lock()
	delete(c.sessions, session)
	c.access.Unlock()
	close(session.done)
	c.server.sessionWg.Done()
}

func (c *sshConn) revoke(reason error) {
	c.server.logger.Info("revoking SSH access for ", c.userProfile.LoginName, ": ", reason)
	c.access.Lock()
	sessions := slices.Collect(maps.Keys(c.sessions))
	c.access.Unlock()
	for _, session := range sessions {
		session.cancel(&terminationError{message: "Access revoked."})
	}
	ctx, cancel := context.WithTimeout(context.Background(), revokeFlushTimeout)
	defer cancel()
	for _, session := range sessions {
		select {
		case <-session.done:
		case <-ctx.Done():
		}
	}
	c.close()
}

func (c *sshConn) recheckPolicy(lookup userLookupFunc) bool {
	c.recheckAccess.Lock()
	defer c.recheckAccess.Unlock()
	rule, err := c.checkPolicy(lookup)
	if err != nil {
		c.revoke(err)
		return false
	}
	c.access.Lock()
	if !c.teardown {
		c.armRuleExpiryLocked(rule)
	}
	c.access.Unlock()
	return true
}

func (c *sshConn) latchedUser(string) (*adapter.PlatformUser, error) {
	return c.localUser, nil
}

func (c *sshConn) checkPolicy(lookup userLookupFunc) (*tailcfg.SSHRule, error) {
	type lookupResult struct {
		user *adapter.PlatformUser
		err  error
	}
	lookups := make(map[string]lookupResult)
	cachedLookup := func(username string) (*adapter.PlatformUser, error) {
		result, loaded := lookups[username]
		if !loaded {
			result.user, result.err = lookup(username)
			lookups[username] = result
		}
		return result.user, result.err
	}
	policy := c.server.sshPolicy()
	if policy == nil {
		return nil, E.New("no SSH policy")
	}
	match, err := c.evaluatePolicy(policy, func(username string) (*adapter.PlatformUser, error) {
		user, err := cachedLookup(username)
		if err != nil && !isUserMissing(err) && username == c.mappedLocalUser {
			return c.localUser, nil
		}
		return user, err
	})
	if err != nil {
		return nil, err
	}
	action := match.action
	switch {
	case action.Reject:
		return nil, E.New("rule changed from accept to reject")
	case action.Accept && action.HoldAndDelegate != "":
		return nil, E.New("malformed action: both accept and hold-and-delegate")
	case !action.Accept && action.HoldAndDelegate == "":
		return nil, E.New("malformed action: neither accept, reject, nor hold-and-delegate")
	}
	if match.localUser != c.mappedLocalUser {
		return nil, E.New("local user changed from ", c.mappedLocalUser, " to ", match.localUser)
	}
	localUser, err := cachedLookup(match.localUser)
	if err != nil {
		if isUserMissing(err) {
			return nil, E.New("local user ", match.localUser, " no longer exists")
		}
		c.server.logger.Warn("SSH policy recheck: lookup local user ", match.localUser, ": ", err, ", retaining identity")
	} else if localUser.Username != c.localUser.Username || localUser.Uid != c.localUser.Uid || localUser.Gid != c.localUser.Gid {
		return nil, E.New("local user ", match.localUser, " identity changed")
	}
	if acceptEnvNarrowed(c.acceptEnv, match.acceptEnv) {
		return nil, E.New("accept-env pattern removed")
	}
	oldRecorders, oldFailure := recordingConfig(c.action0, c.action)
	freshRecorders, freshFailure := recordingConfig(action, action)
	if !action.Accept {
		if c.action0.Accept && (c.latchedRule == nil || !ruleExpired(c.latchedRule)) {
			return nil, E.New("rule changed from accept to hold-and-delegate")
		}
		freshRecorders, freshFailure = recordingConfig(action, c.action)
	}
	if recordersChanged(oldRecorders, freshRecorders) {
		return nil, E.New("session recording obligation changed")
	}
	if recordingFailureChanged(oldFailure, freshFailure) {
		return nil, E.New("recording-failure policy changed")
	}
	if !action.Accept {
		return match.rule, nil
	}
	switch {
	case c.action.AllowLocalPortForwarding && !action.AllowLocalPortForwarding:
		return nil, E.New("local port forwarding revoked")
	case c.action.AllowRemotePortForwarding && !action.AllowRemotePortForwarding:
		return nil, E.New("remote port forwarding revoked")
	case c.action.AllowAgentForwarding && !action.AllowAgentForwarding:
		return nil, E.New("agent forwarding revoked")
	case durationShortened(action.SessionDuration, c.action.SessionDuration):
		return nil, E.New("session duration cap shortened from ", c.action.SessionDuration, " to ", action.SessionDuration)
	}
	return match.rule, nil
}

func recordingConfig(initial, final *tailcfg.SSHAction) ([]netip.AddrPort, *tailcfg.SSHRecorderFailureAction) {
	if len(final.Recorders) > 0 {
		return final.Recorders, final.OnRecordingFailure
	}
	return initial.Recorders, initial.OnRecordingFailure
}

func recordersChanged(old, fresh []netip.AddrPort) bool {
	if len(old) != len(fresh) {
		return true
	}
	counts := make(map[netip.AddrPort]int, len(old))
	for _, recorder := range old {
		counts[recorder]++
	}
	for _, recorder := range fresh {
		if counts[recorder] == 0 {
			return true
		}
		counts[recorder]--
	}
	return false
}

func recordingFailureChanged(old, fresh *tailcfg.SSHRecorderFailureAction) bool {
	var oldAction, freshAction tailcfg.SSHRecorderFailureAction
	if old != nil {
		oldAction = *old
	}
	if fresh != nil {
		freshAction = *fresh
	}
	return oldAction != freshAction
}

func durationShortened(fresh, old time.Duration) bool {
	if fresh == 0 {
		return false
	}
	return old == 0 || fresh < old
}

func acceptEnvNarrowed(old, fresh []string) bool {
	for _, oldPattern := range old {
		if slices.Contains(fresh, oldPattern) {
			continue
		}
		if strings.ContainsAny(oldPattern, "*?") {
			return true
		}
		if !slices.ContainsFunc(fresh, func(freshPattern string) bool {
			return matchAcceptEnvPattern(freshPattern, oldPattern)
		}) {
			return true
		}
	}
	return false
}
