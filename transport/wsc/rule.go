package wsc

import (
	"bytes"
	"errors"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/metadata"
)

type RuleAction int
type RuleDirection int

const (
	RuleActionUnknown RuleAction = iota
	RuleActionReplace
)

const (
	RuleDirectionUnknown RuleDirection = iota
	RuleDirectionInbound
	RuleDirectionOutbound
)

type WSCRule struct {
	Action    RuleAction
	Direction RuleDirection
	Args      []interface{}
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
		var direction RuleDirection = RuleDirectionUnknown
		if len(rule.Direction) > 0 {
			direction, err = RuleDirectionFromString(rule.Direction)
			if err != nil {
				return nil, err
			}
		}
		wscRules = append(wscRules, WSCRule{
			Action:    action,
			Direction: direction,
			Args:      rule.Args,
		})
	}
	return &WSCRuleApplicator{
		Rules: wscRules,
	}, nil
}

func (ruleManager *WSCRuleApplicator) ApplyEndpointReplace(ep string, netw string, direction RuleDirection) (finalEp string, finalNetw string) {
	for _, rule := range ruleManager.Rules {
		if rule.Action != RuleActionReplace {
			continue
		}
		if rule.Direction != RuleDirectionUnknown && direction != RuleDirectionUnknown && rule.Direction != direction {
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
				if whatAddr.IsFqdn() && epAddr.IsFqdn() && whatAddr.Fqdn == epAddr.Fqdn {
					equal = true
				} else if whatAddr.IsIPv4() {
					if epAddr.IsIPv4() || epAddr.Addr.Is4In6() {
						whatAddr4 := whatAddr.Addr.As4()
						epAddr4 := epAddr.Addr.As4()
						equal = bytes.Equal(whatAddr4[:], epAddr4[:])
					}
				} else if whatAddr.IsIPv6() && epAddr.IsIPv6() {
					equal = whatAddr.Addr.Compare(epAddr.Addr) == 0
				}
				if equal {
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
		return RuleActionUnknown, errors.New("rule action doesn't exist")
	}
}

func RuleDirectionFromString(directionStr string) (RuleDirection, error) {
	switch directionStr {
	case "inbound":
		return RuleDirectionInbound, nil
	case "outbound":
		return RuleDirectionOutbound, nil
	default:
		return RuleDirectionUnknown, errors.New("rule direction doesn't exist")
	}
}
