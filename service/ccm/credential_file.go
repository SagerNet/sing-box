package ccm

import (
	"path/filepath"
	"time"

	"github.com/sagernet/fswatch"
	E "github.com/sagernet/sing/common/exceptions"
)

const credentialReloadRetryInterval = 2 * time.Second

func resolveCredentialFilePath(customPath string) (string, error) {
	if customPath == "" {
		var err error
		customPath, err = getDefaultCredentialsPath()
		if err != nil {
			return "", err
		}
	}
	if filepath.IsAbs(customPath) {
		return customPath, nil
	}
	return filepath.Abs(customPath)
}

func (c *defaultCredential) ensureCredentialWatcher() error {
	c.watcherAccess.Lock()
	defer c.watcherAccess.Unlock()

	if c.watcher != nil || c.credentialFilePath == "" {
		return nil
	}
	if !c.watcherRetryAt.IsZero() && time.Now().Before(c.watcherRetryAt) {
		return nil
	}

	watcher, err := fswatch.NewWatcher(fswatch.Options{
		Path:   []string{c.credentialFilePath},
		Logger: c.logger,
		Callback: func(string) {
			err := c.reloadCredentials(true)
			if err != nil {
				c.logger.Warn("reload credentials for ", c.tag, ": ", err)
			}
		},
	})
	if err != nil {
		c.watcherRetryAt = time.Now().Add(credentialReloadRetryInterval)
		return err
	}

	err = watcher.Start()
	if err != nil {
		c.watcherRetryAt = time.Now().Add(credentialReloadRetryInterval)
		return err
	}

	c.watcher = watcher
	c.watcherRetryAt = time.Time{}
	return nil
}

func (c *defaultCredential) retryCredentialReloadIfNeeded() {
	c.stateMutex.RLock()
	unavailable := c.state.unavailable
	lastAttempt := c.state.lastCredentialLoadAttempt
	c.stateMutex.RUnlock()
	if !unavailable {
		return
	}
	if !lastAttempt.IsZero() && time.Since(lastAttempt) < credentialReloadRetryInterval {
		return
	}

	err := c.ensureCredentialWatcher()
	if err != nil {
		c.logger.Debug("start credential watcher for ", c.tag, ": ", err)
	}
	_ = c.reloadCredentials(false)
}

func (c *defaultCredential) reloadCredentials(force bool) error {
	c.reloadAccess.Lock()
	defer c.reloadAccess.Unlock()

	c.stateMutex.RLock()
	unavailable := c.state.unavailable
	lastAttempt := c.state.lastCredentialLoadAttempt
	c.stateMutex.RUnlock()
	if !force {
		if !unavailable {
			return nil
		}
		if !lastAttempt.IsZero() && time.Since(lastAttempt) < credentialReloadRetryInterval {
			return c.unavailableError()
		}
	}

	c.stateMutex.Lock()
	c.state.lastCredentialLoadAttempt = time.Now()
	c.stateMutex.Unlock()

	credentials, err := platformReadCredentials(c.credentialPath)
	if err != nil {
		return c.markCredentialsUnavailable(E.Cause(err, "read credentials"))
	}

	c.accessMutex.Lock()
	c.credentials = credentials
	c.accessMutex.Unlock()

	c.stateMutex.Lock()
	c.state.unavailable = false
	c.state.lastCredentialLoadError = ""
	c.state.accountType = credentials.SubscriptionType
	c.state.rateLimitTier = credentials.RateLimitTier
	c.checkTransitionLocked()
	c.stateMutex.Unlock()

	return nil
}

func (c *defaultCredential) markCredentialsUnavailable(err error) error {
	c.accessMutex.Lock()
	hadCredentials := c.credentials != nil
	c.credentials = nil
	c.accessMutex.Unlock()

	c.stateMutex.Lock()
	c.state.unavailable = true
	c.state.lastCredentialLoadError = err.Error()
	c.state.accountType = ""
	c.state.rateLimitTier = ""
	shouldInterrupt := c.checkTransitionLocked()
	c.stateMutex.Unlock()

	if shouldInterrupt && hadCredentials {
		c.interruptConnections()
	}

	return err
}
