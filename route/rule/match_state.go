package rule

import "github.com/sagernet/sing-box/adapter"

type ruleMatchState uint8

const (
	ruleMatchSourceAddress ruleMatchState = 1 << iota
	ruleMatchSourcePort
	ruleMatchDestinationAddress
	ruleMatchDestinationPort
)

type ruleGroupMatch struct {
	required  ruleMatchState
	satisfied ruleMatchState
}

func (g ruleGroupMatch) done() bool {
	return g.required&^g.satisfied == 0
}

func (g ruleGroupMatch) mergeWith(other ruleGroupMatch) ruleGroupMatch {
	return ruleGroupMatch{
		required:  g.required | other.required,
		satisfied: g.satisfied | other.satisfied,
	}
}

type ruleMatchSnapshot struct {
	ipCidrMatchSource         bool
	ipCidrAcceptEmpty         bool
	deferredIPCIDRMatchGroups uint8
}

func snapshotRuleMatch(metadata *adapter.InboundContext) ruleMatchSnapshot {
	return ruleMatchSnapshot{
		ipCidrMatchSource:         metadata.IPCIDRMatchSource,
		ipCidrAcceptEmpty:         metadata.IPCIDRAcceptEmpty,
		deferredIPCIDRMatchGroups: metadata.DeferredIPCIDRMatchGroups,
	}
}

func (s ruleMatchSnapshot) restore(metadata *adapter.InboundContext) {
	metadata.IPCIDRMatchSource = s.ipCidrMatchSource
	metadata.IPCIDRAcceptEmpty = s.ipCidrAcceptEmpty
	metadata.DeferredIPCIDRMatchGroups = s.deferredIPCIDRMatchGroups
}
