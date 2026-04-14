package oomkiller

import (
	runtimeDebug "runtime/debug"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/byteformats"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/memory"
)

const (
	defaultMinInterval               = 100 * time.Millisecond
	defaultArmedInterval             = time.Second
	defaultMaxInterval               = 10 * time.Second
	defaultSafetyMargin              = 5 * 1024 * 1024
	defaultAvailableTriggerMarginMin = 32 * 1024 * 1024
	defaultAvailableTriggerMarginMax = 128 * 1024 * 1024
)

type pressureState uint8

const (
	pressureStateNormal pressureState = iota
	pressureStateArmed
	pressureStateTriggered
)

type memorySample struct {
	usage          uint64
	available      uint64
	availableKnown bool
}

type pressureThresholds struct {
	trigger uint64
	armed   uint64
	resume  uint64
}

type timerConfig struct {
	memoryLimit     uint64
	safetyMargin    uint64
	hasSafetyMargin bool
	minInterval     time.Duration
	armedInterval   time.Duration
	maxInterval     time.Duration
	policyMode      policyMode
	killerDisabled  bool
}

func buildTimerConfig(options option.OOMKillerServiceOptions, memoryLimit uint64, policyMode policyMode, killerDisabled bool) (timerConfig, error) {
	minInterval := defaultMinInterval
	if options.MinInterval != 0 {
		minInterval = time.Duration(options.MinInterval.Build())
		if minInterval <= 0 {
			return timerConfig{}, E.New("min_interval must be greater than 0")
		}
	}

	maxInterval := defaultMaxInterval
	if options.MaxInterval != 0 {
		maxInterval = time.Duration(options.MaxInterval.Build())
		if maxInterval <= 0 {
			return timerConfig{}, E.New("max_interval must be greater than 0")
		}
	}
	if maxInterval < minInterval {
		return timerConfig{}, E.New("max_interval must be greater than or equal to min_interval")
	}

	var (
		safetyMargin    uint64
		hasSafetyMargin bool
	)
	if options.SafetyMargin != nil && options.SafetyMargin.Value() > 0 {
		safetyMargin = options.SafetyMargin.Value()
		hasSafetyMargin = true
	} else if memoryLimit > 0 {
		safetyMargin = defaultSafetyMargin
		hasSafetyMargin = true
	}

	return timerConfig{
		memoryLimit:     memoryLimit,
		safetyMargin:    safetyMargin,
		hasSafetyMargin: hasSafetyMargin,
		minInterval:     minInterval,
		armedInterval:   max(min(defaultArmedInterval, maxInterval), minInterval),
		maxInterval:     maxInterval,
		policyMode:      policyMode,
		killerDisabled:  killerDisabled,
	}, nil
}

type adaptiveTimer struct {
	timerConfig
	logger          log.ContextLogger
	router          adapter.Router
	onTriggered     func(uint64)
	limitThresholds pressureThresholds

	access                  sync.Mutex
	timer                   *time.Timer
	state                   pressureState
	currentInterval         time.Duration
	forceMinInterval        bool
	pendingPressureBaseline bool
	pressureBaseline        memorySample
	pressureBaselineTime    time.Time
}

func newAdaptiveTimer(logger log.ContextLogger, router adapter.Router, config timerConfig, onTriggered func(uint64)) *adaptiveTimer {
	t := &adaptiveTimer{
		timerConfig: config,
		logger:      logger,
		router:      router,
		onTriggered: onTriggered,
	}
	if config.policyMode == policyModeMemoryLimit || config.policyMode == policyModeNetworkExtension {
		t.limitThresholds = computeLimitThresholds(config.memoryLimit, config.safetyMargin)
	}
	return t
}

func (t *adaptiveTimer) start() {
	t.access.Lock()
	defer t.access.Unlock()
	t.startLocked()
}

func (t *adaptiveTimer) notifyPressure() {
	t.access.Lock()
	t.startLocked()
	t.forceMinInterval = true
	t.pendingPressureBaseline = true
	t.access.Unlock()
	t.poll()
}

func (t *adaptiveTimer) startLocked() {
	if t.timer != nil {
		return
	}
	t.state = pressureStateNormal
	t.forceMinInterval = false
	t.timer = time.AfterFunc(t.minInterval, t.poll)
}

func (t *adaptiveTimer) stop() {
	t.access.Lock()
	defer t.access.Unlock()
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
}

func (t *adaptiveTimer) poll() {
	if t.timerConfig.policyMode == policyModeNetworkExtension {
		runtimeDebug.FreeOSMemory()
	}

	var triggered bool
	var rateTriggered bool
	sample := readMemorySample(t.policyMode)

	t.access.Lock()
	if t.timer == nil {
		t.access.Unlock()
		return
	}
	if t.pendingPressureBaseline {
		t.pressureBaseline = sample
		t.pressureBaselineTime = time.Now()
		t.pendingPressureBaseline = false
	}
	previousState := t.state
	t.state = t.nextState(sample)
	if t.state == pressureStateNormal {
		t.forceMinInterval = false
		if !t.pressureBaselineTime.IsZero() && time.Since(t.pressureBaselineTime) > t.maxInterval {
			t.pressureBaselineTime = time.Time{}
		}
	}
	t.timer.Reset(t.intervalForState())
	triggered = previousState != pressureStateTriggered && t.state == pressureStateTriggered
	if !triggered && !t.pressureBaselineTime.IsZero() && t.memoryLimit > 0 &&
		sample.usage > t.pressureBaseline.usage && sample.usage < t.memoryLimit {
		elapsed := time.Since(t.pressureBaselineTime)
		if elapsed >= t.minInterval/2 {
			growth := sample.usage - t.pressureBaseline.usage
			ratePerSecond := float64(growth) / elapsed.Seconds()
			headroom := t.memoryLimit - sample.usage
			timeToLimit := time.Duration(float64(headroom)/ratePerSecond) * time.Second
			if timeToLimit < t.minInterval {
				triggered = true
				rateTriggered = true
				t.state = pressureStateTriggered
			}
		}
	}
	t.access.Unlock()

	if !triggered {
		return
	}
	t.onTriggered(sample.usage)
	if rateTriggered {
		if t.killerDisabled {
			t.logger.Warn("memory growth rate critical (report only), usage: ", byteformats.FormatMemoryBytes(sample.usage), t.logDetails(sample))
		} else {
			t.logger.Error("memory growth rate critical, usage: ", byteformats.FormatMemoryBytes(sample.usage), t.logDetails(sample), ", resetting network")
			t.router.ResetNetwork()
		}
	} else {
		if t.killerDisabled {
			t.logger.Warn("memory threshold reached (report only), usage: ", byteformats.FormatMemoryBytes(sample.usage), t.logDetails(sample))
		} else {
			t.logger.Error("memory threshold reached, usage: ", byteformats.FormatMemoryBytes(sample.usage), t.logDetails(sample), ", resetting network")
			t.router.ResetNetwork()
		}
	}
	runtimeDebug.FreeOSMemory()
}

func (t *adaptiveTimer) nextState(sample memorySample) pressureState {
	switch t.policyMode {
	case policyModeMemoryLimit, policyModeNetworkExtension:
		return nextPressureState(t.state,
			sample.usage >= t.limitThresholds.trigger,
			sample.usage >= t.limitThresholds.armed,
			sample.usage >= t.limitThresholds.resume,
		)
	case policyModeAvailable:
		if !sample.availableKnown {
			return pressureStateNormal
		}
		thresholds := t.availableThresholds(sample)
		return nextPressureState(t.state,
			sample.available <= thresholds.trigger,
			sample.available <= thresholds.armed,
			sample.available <= thresholds.resume,
		)
	default:
		return pressureStateNormal
	}
}

func computeLimitThresholds(memoryLimit uint64, safetyMargin uint64) pressureThresholds {
	triggerMargin := min(safetyMargin, memoryLimit)
	armedMargin := min(triggerMargin*2, memoryLimit)
	resumeMargin := min(triggerMargin*4, memoryLimit)
	return pressureThresholds{
		trigger: memoryLimit - triggerMargin,
		armed:   memoryLimit - armedMargin,
		resume:  memoryLimit - resumeMargin,
	}
}

func (t *adaptiveTimer) availableThresholds(sample memorySample) pressureThresholds {
	var triggerMargin uint64
	if t.hasSafetyMargin {
		triggerMargin = t.safetyMargin
	} else if sample.usage == 0 {
		triggerMargin = defaultAvailableTriggerMarginMin
	} else {
		triggerMargin = max(defaultAvailableTriggerMarginMin, min(sample.usage/4, defaultAvailableTriggerMarginMax))
	}
	return pressureThresholds{
		trigger: triggerMargin,
		armed:   triggerMargin * 2,
		resume:  triggerMargin * 4,
	}
}

func (t *adaptiveTimer) intervalForState() time.Duration {
	switch {
	case t.forceMinInterval || t.state == pressureStateTriggered:
		t.currentInterval = t.minInterval
	case t.state == pressureStateArmed:
		t.currentInterval = t.armedInterval
	default:
		if t.currentInterval == 0 {
			t.currentInterval = t.maxInterval
		} else {
			t.currentInterval = min(t.currentInterval*2, t.maxInterval)
		}
	}
	return t.currentInterval
}

func (t *adaptiveTimer) logDetails(sample memorySample) string {
	switch t.policyMode {
	case policyModeMemoryLimit, policyModeNetworkExtension:
		headroom := uint64(0)
		if sample.usage < t.memoryLimit {
			headroom = t.memoryLimit - sample.usage
		}
		return ", limit: " + byteformats.FormatMemoryBytes(t.memoryLimit) + ", headroom: " + byteformats.FormatMemoryBytes(headroom)
	case policyModeAvailable:
		if sample.availableKnown {
			return ", available: " + byteformats.FormatMemoryBytes(sample.available)
		}
	}
	return ""
}

func nextPressureState(current pressureState, shouldTrigger, shouldArm, shouldStayTriggered bool) pressureState {
	if current == pressureStateTriggered {
		if shouldStayTriggered {
			return pressureStateTriggered
		}
		return pressureStateNormal
	}
	if shouldTrigger {
		return pressureStateTriggered
	}
	if shouldArm {
		return pressureStateArmed
	}
	return pressureStateNormal
}

func readMemorySample(mode policyMode) memorySample {
	sample := memorySample{
		usage: memory.Total(),
	}
	if mode == policyModeAvailable {
		sample.availableKnown = true
		sample.available = memory.Available()
	}
	return sample
}
