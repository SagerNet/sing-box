package route

import (
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func hasRule(rules []option.Rule, cond func(rule option.DefaultRule) bool) bool {
	for _, rule := range rules {
		switch rule.Type {
		case C.RuleTypeDefault:
			if cond(rule.DefaultOptions) {
				return true
			}
		case C.RuleTypeLogical:
			if hasRule(rule.LogicalOptions.Rules, cond) {
				return true
			}
		}
	}
	return false
}

func hasDNSRule(rules []option.DNSRule, cond func(rule option.DefaultDNSRule) bool) bool {
	for _, rule := range rules {
		switch rule.Type {
		case C.RuleTypeDefault:
			if cond(rule.DefaultOptions) {
				return true
			}
		case C.RuleTypeLogical:
			if hasDNSRule(rule.LogicalOptions.Rules, cond) {
				return true
			}
		}
	}
	return false
}

func isProcessRule(rule option.DefaultRule) bool {
	return len(rule.ProcessName) > 0 || len(rule.ProcessPath) > 0 || len(rule.ProcessPathRegex) > 0 || len(rule.PackageName) > 0 || len(rule.PackageNameRegex) > 0 || len(rule.User) > 0 || len(rule.UserID) > 0
}

func isProcessDNSRule(rule option.DefaultDNSRule) bool {
	return len(rule.ProcessName) > 0 || len(rule.ProcessPath) > 0 || len(rule.ProcessPathRegex) > 0 || len(rule.PackageName) > 0 || len(rule.PackageNameRegex) > 0 || len(rule.User) > 0 || len(rule.UserID) > 0
}

func isNeighborRule(rule option.DefaultRule) bool {
	return len(rule.SourceMACAddress) > 0 || len(rule.SourceHostname) > 0
}

func isNeighborDNSRule(rule option.DefaultDNSRule) bool {
	return len(rule.SourceMACAddress) > 0 || len(rule.SourceHostname) > 0
}

func isWIFIRule(rule option.DefaultRule) bool {
	return len(rule.WIFISSID) > 0 || len(rule.WIFIBSSID) > 0
}

func isWIFIDNSRule(rule option.DefaultDNSRule) bool {
	return len(rule.WIFISSID) > 0 || len(rule.WIFIBSSID) > 0
}

func hasLocalNeighborDNSServer(servers []option.DNSServerOptions) bool {
	for _, server := range servers {
		if server.Type != C.DNSTypeLocal {
			continue
		}
		localOptions, isLocal := server.Options.(*option.LocalDNSServerOptions)
		if !isLocal {
			continue
		}
		if len(localOptions.NeighborDomain) > 0 {
			return true
		}
	}
	return false
}
