package experimental

import (
	"context"
	"os"
	"sort"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
)

type ClashServerConstructor = func(ctx context.Context, logFactory log.ObservableFactory, options option.ClashAPIOptions) (adapter.ClashServer, error)

var clashServerConstructor ClashServerConstructor

func RegisterClashServerConstructor(constructor ClashServerConstructor) {
	clashServerConstructor = constructor
}

func NewClashServer(ctx context.Context, logFactory log.ObservableFactory, options option.ClashAPIOptions) (adapter.ClashServer, error) {
	if clashServerConstructor == nil {
		return nil, os.ErrInvalid
	}
	return clashServerConstructor(ctx, logFactory, options)
}

func CalculateClashModeList(options option.Options) []string {
	var clashModes []string
	clashModes = append(clashModes, extraClashModeFromRule(common.PtrValueOrDefault(options.Route).Rules)...)
	clashModes = append(clashModes, extraClashModeFromDNSRule(common.PtrValueOrDefault(options.DNS).Rules)...)
	clashModes = common.FilterNotDefault(common.Uniq(clashModes))
	predefinedOrder := []string{
		"Rule", "Global", "Direct",
	}
	var newClashModes []string
	for _, mode := range clashModes {
		if !common.Contains(predefinedOrder, mode) {
			newClashModes = append(newClashModes, mode)
		}
	}
	sort.Strings(newClashModes)
	for _, mode := range predefinedOrder {
		if common.Contains(clashModes, mode) {
			newClashModes = append(newClashModes, mode)
		}
	}
	return newClashModes
}

func extraClashModeFromRule(rules []option.Rule) []string {
	var clashMode []string
	for _, rule := range rules {
		switch rule.Type {
		case C.RuleTypeDefault:
			if rule.DefaultOptions.ClashMode != "" {
				clashMode = append(clashMode, rule.DefaultOptions.ClashMode)
			}
		case C.RuleTypeLogical:
			clashMode = append(clashMode, extraClashModeFromRule(rule.LogicalOptions.Rules)...)
		}
	}
	return clashMode
}

func extraClashModeFromDNSRule(rules []option.DNSRule) []string {
	var clashMode []string
	for _, rule := range rules {
		switch rule.Type {
		case C.RuleTypeDefault:
			if rule.DefaultOptions.ClashMode != "" {
				clashMode = append(clashMode, rule.DefaultOptions.ClashMode)
			}
		case C.RuleTypeLogical:
			clashMode = append(clashMode, extraClashModeFromDNSRule(rule.LogicalOptions.Rules)...)
		}
	}
	return clashMode
}
