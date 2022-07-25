package canceler

import (
	"context"
	"time"
)

type Instance struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	timer      *time.Timer
	timeout    time.Duration
}

func New(ctx context.Context, cancelFunc context.CancelFunc, timeout time.Duration) *Instance {
	instance := &Instance{
		ctx,
		cancelFunc,
		time.NewTimer(timeout),
		timeout,
	}
	go instance.wait()
	return instance
}

func (i *Instance) Update() bool {
	if !i.timer.Stop() {
		return false
	}
	if !i.timer.Reset(i.timeout) {
		return false
	}
	return true
}

func (i *Instance) wait() {
	select {
	case <-i.timer.C:
	case <-i.ctx.Done():
	}
	i.Close()
}

func (i *Instance) Close() error {
	i.timer.Stop()
	i.cancelFunc()
	return nil
}
