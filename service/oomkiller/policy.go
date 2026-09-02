package oomkiller

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/memory"
	"github.com/sagernet/sing/service"
)

const (
	DefaultAppleNetworkExtensionMemoryLimit = 50 * 1024 * 1024
	DefaultAppleNetworkExtensionGCPercent   = 50
)

type policyMode uint8

const (
	policyModeNone policyMode = iota
	policyModeMemoryLimit
	policyModeAvailable
	policyModeNetworkExtension
)

func (m policyMode) hasTimerMode() bool {
	return m != policyModeNone
}

func (m policyMode) String() string {
	switch m {
	case policyModeMemoryLimit:
		return "memory_limit"
	case policyModeAvailable:
		return "available"
	case policyModeNetworkExtension:
		return "network_extension"
	default:
		return "none"
	}
}

func resolvePolicyMode(ctx context.Context, options option.OOMKillerServiceOptions) (uint64, policyMode) {
	platformInterface := service.FromContext[adapter.PlatformInterface](ctx)
	if C.IsIos && platformInterface != nil && platformInterface.UnderNetworkExtension() {
		return DefaultAppleNetworkExtensionMemoryLimit, policyModeNetworkExtension
	}
	if options.MemoryLimitOverride > 0 {
		return options.MemoryLimitOverride, policyModeMemoryLimit
	}
	if options.MemoryLimit != nil {
		memoryLimit := options.MemoryLimit.Value()
		if memoryLimit > 0 {
			return memoryLimit, policyModeMemoryLimit
		}
	}
	if memory.AvailableAvailable() {
		return 0, policyModeAvailable
	}
	return 0, policyModeNone
}
