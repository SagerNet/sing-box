package v2raykcp

import (
	"sync/atomic"
	"time"
)

type Updater struct {
	interval        int64
	shouldContinue  func() bool
	shouldTerminate func() bool
	updateFunc      func()
	notifier        chan struct{}
}

func NewUpdater(interval uint32, shouldContinue func() bool, shouldTerminate func() bool, updateFunc func()) *Updater {
	u := &Updater{
		interval:        int64(time.Duration(interval) * time.Millisecond),
		shouldContinue:  shouldContinue,
		shouldTerminate: shouldTerminate,
		updateFunc:      updateFunc,
		notifier:        make(chan struct{}, 1),
	}
	return u
}

func (u *Updater) WakeUp() {
	select {
	case u.notifier <- struct{}{}:
		go u.run()
	default:
	}
}

func (u *Updater) run() {
	defer func() {
		<-u.notifier
	}()

	if u.shouldTerminate() {
		return
	}
	ticker := time.NewTicker(u.Interval())
	defer ticker.Stop()
	
	for u.shouldContinue() {
		u.updateFunc()
		<-ticker.C
	}
}

func (u *Updater) Interval() time.Duration {
	return time.Duration(atomic.LoadInt64(&u.interval))
}

func (u *Updater) SetInterval(d time.Duration) {
	atomic.StoreInt64(&u.interval, int64(d))
}
