package wsc

import (
	"errors"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/metadata"
)

type RuleAction int

const (
	RuleActionUnknown RuleAction = iota
	RuleActionReplace
)

type WSCRule struct {
	Action RuleAction
	Args   []interface{}
}

type WSCRuleApplicator struct {
	Rules []WSCRule
}

func NewRuleApplicator(rules []option.WSCRule) (*WSCRuleApplicator, error) {
	wscRules := make([]WSCRule, 0, len(rules))
	for _, rule := range rules {
		action, err := RuleActionFromString(rule.Action)
		if err != nil {
			return nil, err
		}
		wscRules = append(wscRules, WSCRule{
			Action: action,
			Args:   rule.Args,
		})
	}
	return &WSCRuleApplicator{
		Rules: wscRules,
	}, nil
}

func (ruleManager *WSCRuleApplicator) ApplyEndpointReplace(ep string, netw string) (finalEp string, finalNetw string) {
	finalEp, finalNetw = ep, netw

	for _, rule := range ruleManager.Rules {
		if rule.Action != RuleActionReplace {
			continue
		}

		sType, ok := rule.Args[0].(string)
		if !ok {
			continue
		}
		what, ok := rule.Args[1].(string)
		if !ok {
			continue
		}
		with, ok := rule.Args[2].(string)
		if !ok {
			continue
		}

		switch sType {
		case "endpoint":
			{
				var proto string
				var protoWith string
				var protoOk bool = false
				var protoWithOk bool = false
				if len(rule.Args) > 3 {
					proto, protoOk = rule.Args[3].(string)
				}
				if len(rule.Args) > 4 {
					protoWith, protoWithOk = rule.Args[4].(string)
				}

				whatAddr := metadata.ParseSocksaddr(what)
				withAddr := metadata.ParseSocksaddr(with)
				epAddr := metadata.ParseSocksaddr(ep)

				equal := false
				if (whatAddr.IsFqdn() && epAddr.IsFqdn() && whatAddr.Fqdn == epAddr.Fqdn) || whatAddr.Addr.Compare(epAddr.Addr) == 0 {
					if whatAddr.Port == 0 {
						equal = true
					} else {
						equal = whatAddr.Port == epAddr.Port
					}
				}
				if protoOk {
					equal = equal && netw == proto
				}

				if equal {
					port := withAddr.Port
					if port == 0 {
						port = epAddr.Port
					}
					ep = (metadata.Socksaddr{
						Addr: withAddr.Addr,
						Port: port,
						Fqdn: withAddr.Fqdn,
					}).String()
					if protoWithOk {
						netw = protoWith
					}
				}
			}
		}
	}

	return ep, netw
}

func RuleActionFromString(actionStr string) (RuleAction, error) {
	switch actionStr {
	case "replace":
		return RuleActionReplace, nil
	default:
		return 0, errors.New("rule action doesn't exist")
	}
}
