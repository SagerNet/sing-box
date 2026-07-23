package rule

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
