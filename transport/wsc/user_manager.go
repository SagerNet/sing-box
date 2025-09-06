package wsc

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

type wscUserManager struct {
	mu                         sync.Mutex
	users                      map[int64]*wscUser
	maxConnPerUser             int
	usageReportTrafficInterval int64
	usageReportTimeInterval    time.Duration
	authenticator              Authenticator
}

func (manager *wscUserManager) findOrCreateUser(ctx context.Context, uid int64, rateLimit int64, maxConn int) *wscUser {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if user, exists := manager.users[uid]; exists {
		manager.reportUser(ctx, user, false)
		return user
	}
	user := manager.newUser(uid, 0, min(manager.maxConnPerUser, maxConn), rateLimit)
	manager.users[uid] = user
	return user
}

func (manager *wscUserManager) reportUser(ctx context.Context, user *wscUser, force bool) bool {
	usedTraffic := user.usedTrafficBytes.Load()
	reportedTraffic := user.reportedTrafficBytes.Load()
	trafficResult := usedTraffic - reportedTraffic
	now := nowns()

	if !force {
		if trafficResult == 0 {
			return false
		}
		if trafficResult < manager.usageReportTrafficInterval && time.Duration(now-user.lastTrafficUpdateTick.Load()) < manager.usageReportTimeInterval {
			return false
		}
	}

	go func() {
		_, err := manager.authenticator.ReportUsage(ctx, ReportUsageParams{
			ID:          user.id,
			UsedTraffic: trafficResult,
		})
		if err == nil {
			user.reportedTrafficBytes.Store(usedTraffic)
			user.lastTrafficUpdateTick.Store(nowns())
		}
	}()

	return true
}

func (manager *wscUserManager) cleanupUserConn(ctx context.Context, user *wscUser, conn net.Conn) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	sent := manager.reportUser(ctx, user, false)
	err := user.removeConn(conn)
	if user.connCount() == 0 {
		if !sent {
			manager.reportUser(ctx, user, true)
		}
		delete(manager.users, user.id)
	}
	return err
}

func (manager *wscUserManager) cleanupUser(ctx context.Context, uid int64, forceReport bool) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if user, exists := manager.users[uid]; !exists {
		return errors.New("user doesn't exist")
	} else {
		manager.reportUser(ctx, user, forceReport)
		user.cleanup()
		delete(manager.users, uid)
		return nil
	}
}
