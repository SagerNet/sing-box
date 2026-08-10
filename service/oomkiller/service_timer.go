package oomkiller

import (
	runtimeDebug "runtime/debug"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/memory"
)

const (
	defaultChecksBeforeLimit = 4
	defaultMinInterval       = 500 * time.Millisecond
	defaultMaxInterval       = 10 * time.Second
	defaultSafetyMargin      = 5 * 1024 * 1024
)

type adaptiveTimer struct {
	logger            log.ContextLogger
	network           adapter.NetworkManager
	memoryLimit       uint64
	safetyMargin      uint64
	minInterval       time.Duration
	maxInterval       time.Duration
	checksBeforeLimit int
	useAvailable      bool

	access        sync.Mutex
	timer         *time.Timer
	previousUsage uint64
	lastInterval  time.Duration
}

type timerConfig struct {
	memoryLimit       uint64
	safetyMargin      uint64
	minInterval       time.Duration
	maxInterval       time.Duration
	checksBeforeLimit int
	useAvailable      bool
}

func newAdaptiveTimer(logger log.ContextLogger, network adapter.NetworkManager, config timerConfig) *adaptiveTimer {
	return &adaptiveTimer{
		logger:            logger,
		network:           network,
		memoryLimit:       config.memoryLimit,
		safetyMargin:      config.safetyMargin,
		minInterval:       config.minInterval,
		maxInterval:       config.maxInterval,
		checksBeforeLimit: config.checksBeforeLimit,
		useAvailable:      config.useAvailable,
	}
}

func (t *adaptiveTimer) start(immediate bool) {
	t.access.Lock()
	t.startLocked()
	t.access.Unlock()
	if immediate {
		t.poll()
	}
}

func (t *adaptiveTimer) startLocked() {
	if t.timer != nil {
		return
	}
	t.previousUsage = memory.Total()
	t.lastInterval = t.minInterval
	t.timer = time.AfterFunc(t.minInterval, t.poll)
}

func (t *adaptiveTimer) stop() {
	t.access.Lock()
	defer t.access.Unlock()
	t.stopLocked()
}

func (t *adaptiveTimer) stopLocked() {
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
}

func (t *adaptiveTimer) poll() {
	t.access.Lock()
	defer t.access.Unlock()
	if t.timer == nil {
		return
	}

	usage := memory.Total()
	delta := int64(usage) - int64(t.previousUsage)
	t.previousUsage = usage

	var remaining uint64
	var triggered bool

	if t.memoryLimit > 0 {
		if usage >= t.memoryLimit {
			remaining = 0
			triggered = true
		} else {
			remaining = t.memoryLimit - usage
		}
	} else if t.useAvailable {
		available := memory.Available()
		if available <= t.safetyMargin {
			remaining = 0
			triggered = true
		} else {
			remaining = available - t.safetyMargin
		}
	} else {
		remaining = 0
	}

	if triggered {
		t.logger.Error("memory threshold reached, usage: ", usage/(1024*1024), " MiB, resetting network")
		t.network.ResetNetwork()
		runtimeDebug.FreeOSMemory()
	}

	var interval time.Duration
	if triggered {
		interval = t.maxInterval
	} else if delta <= 0 {
		interval = t.maxInterval
	} else if t.checksBeforeLimit <= 0 {
		interval = t.maxInterval
	} else {
		timeToLimit := time.Duration(float64(remaining) / float64(delta) * float64(t.lastInterval))
		interval = max(timeToLimit/time.Duration(t.checksBeforeLimit), t.minInterval)
		interval = min(interval, t.maxInterval)
	}

	t.lastInterval = interval
	t.timer.Reset(interval)
}
