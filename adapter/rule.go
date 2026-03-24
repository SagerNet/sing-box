package adapter

import (
	C "github.com/sagernet/sing-box/constant"

	"github.com/miekg/dns"
)

type HeadlessRule interface {
	Match(metadata *InboundContext) bool
	String() string
}

type Rule interface {
	HeadlessRule
	SimpleLifecycle
	Type() string
	Action() RuleAction
}

type DNSRule interface {
	Rule
	WithAddressLimit() bool
	MatchAddressLimit(metadata *InboundContext, response *dns.Msg) bool
}

type RuleAction interface {
	Type() string
	String() string
}

func IsFinalAction(action RuleAction) bool {
	switch action.Type() {
	case C.RuleActionTypeSniff, C.RuleActionTypeResolve:
		return false
	default:
		return true
	}
}
