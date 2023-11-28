package experimental

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
)

type ClashServerConstructor = func(ctx context.Context, router adapter.Router, logFactory log.ObservableFactory, options option.ClashAPIOptions) (adapter.ClashServer, error)

var clashServerConstructor ClashServerConstructor

func RegisterClashServerConstructor(constructor ClashServerConstructor) {
	clashServerConstructor = constructor
}

func NewClashServer(ctx context.Context, router adapter.Router, logFactory log.ObservableFactory, options option.ClashAPIOptions) (adapter.ClashServer, error) {
	if clashServerConstructor == nil {
		return nil, os.ErrInvalid
	}
	return clashServerConstructor(ctx, router, logFactory, options)
}

func CalculateClashModeList(options option.Options) []string {
	var clashMode []string
	clashMode = append(clashMode, extraClashModeFromRule(common.PtrValueOrDefault(options.Route).Rules)...)
	clashMode = append(clashMode, extraClashModeFromDNSRule(common.PtrValueOrDefault(options.DNS).Rules)...)
	clashMode = common.FilterNotDefault(common.Uniq(clashMode))
	return clashMode
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
