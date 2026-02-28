package oomkiller

import (
	"time"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func buildTimerConfig(options option.OOMKillerServiceOptions, memoryLimit uint64, useAvailable bool) (timerConfig, error) {
	safetyMargin := uint64(defaultSafetyMargin)
	if options.SafetyMargin != nil && options.SafetyMargin.Value() > 0 {
		safetyMargin = options.SafetyMargin.Value()
	}

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

	checksBeforeLimit := defaultChecksBeforeLimit
	if options.ChecksBeforeLimit != 0 {
		checksBeforeLimit = options.ChecksBeforeLimit
		if checksBeforeLimit <= 0 {
			return timerConfig{}, E.New("checks_before_limit must be greater than 0")
		}
	}

	return timerConfig{
		memoryLimit:       memoryLimit,
		safetyMargin:      safetyMargin,
		minInterval:       minInterval,
		maxInterval:       maxInterval,
		checksBeforeLimit: checksBeforeLimit,
		useAvailable:      useAvailable,
	}, nil
}
