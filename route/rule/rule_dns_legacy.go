package rule

import (
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"

	"go4.org/netipx"
)

type legacyResponseLiteralKind uint8

const (
	legacyLiteralRequireEmpty legacyResponseLiteralKind = iota
	legacyLiteralRequireNonEmpty
	legacyLiteralRequireSet
	legacyLiteralForbidSet
)

type legacyResponseLiteral struct {
	kind  legacyResponseLiteralKind
	ipSet *netipx.IPSet
}

type legacyResponseFormulaKind uint8

const (
	legacyFormulaFalse legacyResponseFormulaKind = iota
	legacyFormulaTrue
	legacyFormulaLiteral
	legacyFormulaAnd
	legacyFormulaOr
)

type legacyResponseFormula struct {
	kind     legacyResponseFormulaKind
	literal  legacyResponseLiteral
	children []legacyResponseFormula
}

type legacyResponseConstraint struct {
	requireEmpty    bool
	requireNonEmpty bool
	requiredSets    []*netipx.IPSet
	forbiddenSet    *netipx.IPSet
}

type legacyRuleMatchStateSet [16]legacyResponseFormula

var (
	legacyAllIPSet = func() *netipx.IPSet {
		var builder netipx.IPSetBuilder
		builder.Complement()
		return common.Must1(builder.IPSet())
	}()
	legacyNonPublicIPSet = func() *netipx.IPSet {
		var builder netipx.IPSetBuilder
		for _, prefix := range []string{
			"0.0.0.0/32",
			"10.0.0.0/8",
			"127.0.0.0/8",
			"169.254.0.0/16",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"224.0.0.0/4",
			"::/128",
			"::1/128",
			"fc00::/7",
			"fe80::/10",
			"ff00::/8",
		} {
			builder.AddPrefix(netip.MustParsePrefix(prefix))
		}
		return common.Must1(builder.IPSet())
	}()
)

func legacyFalseFormula() legacyResponseFormula {
	return legacyResponseFormula{}
}

func legacyTrueFormula() legacyResponseFormula {
	return legacyResponseFormula{kind: legacyFormulaTrue}
}

func legacyLiteralFormula(literal legacyResponseLiteral) legacyResponseFormula {
	return legacyResponseFormula{
		kind:    legacyFormulaLiteral,
		literal: literal,
	}
}

func (f legacyResponseFormula) isFalse() bool {
	return f.kind == legacyFormulaFalse
}

func (f legacyResponseFormula) isTrue() bool {
	return f.kind == legacyFormulaTrue
}

func (f legacyResponseFormula) or(other legacyResponseFormula) legacyResponseFormula {
	return legacyOrFormulas(f, other)
}

func (f legacyResponseFormula) and(other legacyResponseFormula) legacyResponseFormula {
	return legacyAndFormulas(f, other)
}

func (f legacyResponseFormula) not() legacyResponseFormula {
	switch f.kind {
	case legacyFormulaFalse:
		return legacyTrueFormula()
	case legacyFormulaTrue:
		return legacyFalseFormula()
	case legacyFormulaLiteral:
		return legacyLiteralFormula(legacyNegateResponseLiteral(f.literal))
	case legacyFormulaAnd:
		negated := make([]legacyResponseFormula, 0, len(f.children))
		for _, child := range f.children {
			negated = append(negated, child.not())
		}
		return legacyOrFormulas(negated...)
	case legacyFormulaOr:
		negated := make([]legacyResponseFormula, 0, len(f.children))
		for _, child := range f.children {
			negated = append(negated, child.not())
		}
		return legacyAndFormulas(negated...)
	default:
		panic("unknown legacy response formula kind")
	}
}

func legacyNegateResponseLiteral(literal legacyResponseLiteral) legacyResponseLiteral {
	switch literal.kind {
	case legacyLiteralRequireEmpty:
		return legacyResponseLiteral{kind: legacyLiteralRequireNonEmpty}
	case legacyLiteralRequireNonEmpty:
		return legacyResponseLiteral{kind: legacyLiteralRequireEmpty}
	case legacyLiteralRequireSet:
		return legacyResponseLiteral{kind: legacyLiteralForbidSet, ipSet: literal.ipSet}
	case legacyLiteralForbidSet:
		return legacyResponseLiteral{kind: legacyLiteralRequireSet, ipSet: literal.ipSet}
	default:
		panic("unknown legacy response literal kind")
	}
}

func legacyOrFormulas(formulas ...legacyResponseFormula) legacyResponseFormula {
	children := make([]legacyResponseFormula, 0, len(formulas))
	for _, formula := range formulas {
		if formula.isFalse() {
			continue
		}
		if formula.isTrue() {
			return legacyTrueFormula()
		}
		if formula.kind == legacyFormulaOr {
			children = append(children, formula.children...)
			continue
		}
		children = append(children, formula)
	}
	switch len(children) {
	case 0:
		return legacyFalseFormula()
	case 1:
		return children[0]
	default:
		return legacyResponseFormula{
			kind:     legacyFormulaOr,
			children: children,
		}
	}
}

func legacyAndFormulas(formulas ...legacyResponseFormula) legacyResponseFormula {
	children := make([]legacyResponseFormula, 0, len(formulas))
	for _, formula := range formulas {
		if formula.isFalse() {
			return legacyFalseFormula()
		}
		if formula.isTrue() {
			continue
		}
		if formula.kind == legacyFormulaAnd {
			children = append(children, formula.children...)
			continue
		}
		children = append(children, formula)
	}
	switch len(children) {
	case 0:
		return legacyTrueFormula()
	case 1:
		return children[0]
	}
	result := legacyResponseFormula{
		kind:     legacyFormulaAnd,
		children: children,
	}
	if !result.satisfiable() {
		return legacyFalseFormula()
	}
	return result
}

func (f legacyResponseFormula) satisfiable() bool {
	return legacyResponseFormulasSatisfiable(legacyResponseConstraint{}, []legacyResponseFormula{f})
}

func legacyResponseFormulasSatisfiable(constraint legacyResponseConstraint, formulas []legacyResponseFormula) bool {
	stack := append(make([]legacyResponseFormula, 0, len(formulas)), formulas...)
	var disjunctions []legacyResponseFormula
	for len(stack) > 0 {
		formula := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch formula.kind {
		case legacyFormulaFalse:
			return false
		case legacyFormulaTrue:
			continue
		case legacyFormulaLiteral:
			var ok bool
			constraint, ok = constraint.withLiteral(formula.literal)
			if !ok {
				return false
			}
		case legacyFormulaAnd:
			stack = append(stack, formula.children...)
		case legacyFormulaOr:
			if len(formula.children) == 0 {
				return false
			}
			disjunctions = append(disjunctions, formula)
		default:
			panic("unknown legacy response formula kind")
		}
	}
	if len(disjunctions) == 0 {
		return true
	}
	bestIndex := 0
	for i := 1; i < len(disjunctions); i++ {
		if len(disjunctions[i].children) < len(disjunctions[bestIndex].children) {
			bestIndex = i
		}
	}
	selected := disjunctions[bestIndex]
	remaining := make([]legacyResponseFormula, 0, len(disjunctions)-1)
	remaining = append(remaining, disjunctions[:bestIndex]...)
	remaining = append(remaining, disjunctions[bestIndex+1:]...)
	for _, child := range selected.children {
		nextFormulas := make([]legacyResponseFormula, 0, len(remaining)+1)
		nextFormulas = append(nextFormulas, remaining...)
		nextFormulas = append(nextFormulas, child)
		if legacyResponseFormulasSatisfiable(constraint, nextFormulas) {
			return true
		}
	}
	return false
}

func (c legacyResponseConstraint) withLiteral(literal legacyResponseLiteral) (legacyResponseConstraint, bool) {
	switch literal.kind {
	case legacyLiteralRequireEmpty:
		c.requireEmpty = true
	case legacyLiteralRequireNonEmpty:
		c.requireNonEmpty = true
	case legacyLiteralRequireSet:
		requiredSets := make([]*netipx.IPSet, len(c.requiredSets)+1)
		copy(requiredSets, c.requiredSets)
		requiredSets[len(c.requiredSets)] = literal.ipSet
		c.requiredSets = requiredSets
	case legacyLiteralForbidSet:
		c.forbiddenSet = legacyUnionIPSets(c.forbiddenSet, literal.ipSet)
	default:
		panic("unknown legacy response literal kind")
	}
	return c, c.satisfiable()
}

func (c legacyResponseConstraint) satisfiable() bool {
	if c.requireEmpty && (c.requireNonEmpty || len(c.requiredSets) > 0) {
		return false
	}
	if c.requireEmpty {
		return true
	}
	for _, required := range c.requiredSets {
		if !legacyIPSetHasAllowedIP(required, c.forbiddenSet) {
			return false
		}
	}
	if c.requireNonEmpty && len(c.requiredSets) == 0 {
		return legacyIPSetHasAllowedIP(legacyAllIPSet, c.forbiddenSet)
	}
	return true
}

func legacyUnionIPSets(left *netipx.IPSet, right *netipx.IPSet) *netipx.IPSet {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	var builder netipx.IPSetBuilder
	builder.AddSet(left)
	builder.AddSet(right)
	return common.Must1(builder.IPSet())
}

func legacyIPSetHasAllowedIP(required *netipx.IPSet, forbidden *netipx.IPSet) bool {
	if required == nil {
		required = legacyAllIPSet
	}
	if forbidden == nil {
		return len(required.Ranges()) > 0
	}
	builder := netipx.IPSetBuilder{}
	builder.AddSet(required)
	builder.RemoveSet(forbidden)
	remaining := common.Must1(builder.IPSet())
	return len(remaining.Ranges()) > 0
}

func legacySingleRuleMatchState(state ruleMatchState) legacyRuleMatchStateSet {
	return legacySingleRuleMatchStateWithFormula(state, legacyTrueFormula())
}

func legacySingleRuleMatchStateWithFormula(state ruleMatchState, formula legacyResponseFormula) legacyRuleMatchStateSet {
	var stateSet legacyRuleMatchStateSet
	if !formula.isFalse() {
		stateSet[state] = formula
	}
	return stateSet
}

func (s legacyRuleMatchStateSet) isEmpty() bool {
	for _, formula := range s {
		if !formula.isFalse() {
			return false
		}
	}
	return true
}

func (s legacyRuleMatchStateSet) merge(other legacyRuleMatchStateSet) legacyRuleMatchStateSet {
	var merged legacyRuleMatchStateSet
	for state := ruleMatchState(0); state < 16; state++ {
		merged[state] = s[state].or(other[state])
	}
	return merged
}

func (s legacyRuleMatchStateSet) combine(other legacyRuleMatchStateSet) legacyRuleMatchStateSet {
	if s.isEmpty() || other.isEmpty() {
		return legacyRuleMatchStateSet{}
	}
	var combined legacyRuleMatchStateSet
	for left := ruleMatchState(0); left < 16; left++ {
		if s[left].isFalse() {
			continue
		}
		for right := ruleMatchState(0); right < 16; right++ {
			if other[right].isFalse() {
				continue
			}
			combined[left|right] = combined[left|right].or(s[left].and(other[right]))
		}
	}
	return combined
}

func (s legacyRuleMatchStateSet) withBase(base ruleMatchState) legacyRuleMatchStateSet {
	if s.isEmpty() {
		return legacyRuleMatchStateSet{}
	}
	var withBase legacyRuleMatchStateSet
	for state := ruleMatchState(0); state < 16; state++ {
		if s[state].isFalse() {
			continue
		}
		withBase[state|base] = withBase[state|base].or(s[state])
	}
	return withBase
}

func (s legacyRuleMatchStateSet) filter(allowed func(ruleMatchState) bool) legacyRuleMatchStateSet {
	var filtered legacyRuleMatchStateSet
	for state := ruleMatchState(0); state < 16; state++ {
		if s[state].isFalse() {
			continue
		}
		if allowed(state) {
			filtered[state] = s[state]
		}
	}
	return filtered
}

func (s legacyRuleMatchStateSet) addBit(bit ruleMatchState) legacyRuleMatchStateSet {
	var withBit legacyRuleMatchStateSet
	for state := ruleMatchState(0); state < 16; state++ {
		if s[state].isFalse() {
			continue
		}
		withBit[state|bit] = withBit[state|bit].or(s[state])
	}
	return withBit
}

func (s legacyRuleMatchStateSet) branchOnBit(bit ruleMatchState, condition legacyResponseFormula) legacyRuleMatchStateSet {
	if condition.isFalse() {
		return s
	}
	if condition.isTrue() {
		return s.addBit(bit)
	}
	var branched legacyRuleMatchStateSet
	conditionFalse := condition.not()
	for state := ruleMatchState(0); state < 16; state++ {
		if s[state].isFalse() {
			continue
		}
		if state.has(bit) {
			branched[state] = branched[state].or(s[state])
			continue
		}
		branched[state] = branched[state].or(s[state].and(conditionFalse))
		branched[state|bit] = branched[state|bit].or(s[state].and(condition))
	}
	return branched
}

func (s legacyRuleMatchStateSet) andFormula(formula legacyResponseFormula) legacyRuleMatchStateSet {
	if formula.isFalse() || s.isEmpty() {
		return legacyRuleMatchStateSet{}
	}
	if formula.isTrue() {
		return s
	}
	var result legacyRuleMatchStateSet
	for state := ruleMatchState(0); state < 16; state++ {
		if s[state].isFalse() {
			continue
		}
		result[state] = s[state].and(formula)
	}
	return result
}

func (s legacyRuleMatchStateSet) anyFormula() legacyResponseFormula {
	var formula legacyResponseFormula
	for _, stateFormula := range s {
		formula = formula.or(stateFormula)
	}
	return formula
}

type legacyRuleStateMatcher interface {
	legacyMatchStates(metadata *adapter.InboundContext) legacyRuleMatchStateSet
}

type legacyRuleStateMatcherWithBase interface {
	legacyMatchStatesWithBase(metadata *adapter.InboundContext, base ruleMatchState) legacyRuleMatchStateSet
}

func legacyMatchHeadlessRuleStates(rule adapter.HeadlessRule, metadata *adapter.InboundContext) legacyRuleMatchStateSet {
	return legacyMatchHeadlessRuleStatesWithBase(rule, metadata, 0)
}

func legacyMatchHeadlessRuleStatesWithBase(rule adapter.HeadlessRule, metadata *adapter.InboundContext, base ruleMatchState) legacyRuleMatchStateSet {
	if matcher, loaded := rule.(legacyRuleStateMatcherWithBase); loaded {
		return matcher.legacyMatchStatesWithBase(metadata, base)
	}
	if matcher, loaded := rule.(legacyRuleStateMatcher); loaded {
		return matcher.legacyMatchStates(metadata).withBase(base)
	}
	if rule.Match(metadata) {
		return legacySingleRuleMatchState(base)
	}
	return legacyRuleMatchStateSet{}
}

func legacyMatchRuleItemStatesWithBase(item RuleItem, metadata *adapter.InboundContext, base ruleMatchState) legacyRuleMatchStateSet {
	if matcher, loaded := item.(legacyRuleStateMatcherWithBase); loaded {
		return matcher.legacyMatchStatesWithBase(metadata, base)
	}
	if matcher, loaded := item.(legacyRuleStateMatcher); loaded {
		return matcher.legacyMatchStates(metadata).withBase(base)
	}
	if item.Match(metadata) {
		return legacySingleRuleMatchState(base)
	}
	return legacyRuleMatchStateSet{}
}

func (r *DefaultHeadlessRule) legacyMatchStates(metadata *adapter.InboundContext) legacyRuleMatchStateSet {
	return r.abstractDefaultRule.legacyMatchStates(metadata)
}

func (r *LogicalHeadlessRule) legacyMatchStates(metadata *adapter.InboundContext) legacyRuleMatchStateSet {
	return r.abstractLogicalRule.legacyMatchStates(metadata)
}

func (r *RuleSetItem) legacyMatchStates(metadata *adapter.InboundContext) legacyRuleMatchStateSet {
	return r.legacyMatchStatesWithBase(metadata, 0)
}

func (r *RuleSetItem) legacyMatchStatesWithBase(metadata *adapter.InboundContext, base ruleMatchState) legacyRuleMatchStateSet {
	var stateSet legacyRuleMatchStateSet
	for _, ruleSet := range r.setList {
		nestedMetadata := *metadata
		nestedMetadata.ResetRuleMatchCache()
		nestedMetadata.IPCIDRMatchSource = r.ipCidrMatchSource
		nestedMetadata.IPCIDRAcceptEmpty = r.ipCidrAcceptEmpty
		stateSet = stateSet.merge(legacyMatchHeadlessRuleStatesWithBase(ruleSet, &nestedMetadata, base))
	}
	return stateSet
}

func (s *LocalRuleSet) legacyMatchStates(metadata *adapter.InboundContext) legacyRuleMatchStateSet {
	return s.legacyMatchStatesWithBase(metadata, 0)
}

func (s *LocalRuleSet) legacyMatchStatesWithBase(metadata *adapter.InboundContext, base ruleMatchState) legacyRuleMatchStateSet {
	var stateSet legacyRuleMatchStateSet
	for _, rule := range s.rules {
		nestedMetadata := *metadata
		nestedMetadata.ResetRuleMatchCache()
		stateSet = stateSet.merge(legacyMatchHeadlessRuleStatesWithBase(rule, &nestedMetadata, base))
	}
	return stateSet
}

func (s *RemoteRuleSet) legacyMatchStates(metadata *adapter.InboundContext) legacyRuleMatchStateSet {
	return s.legacyMatchStatesWithBase(metadata, 0)
}

func (s *RemoteRuleSet) legacyMatchStatesWithBase(metadata *adapter.InboundContext, base ruleMatchState) legacyRuleMatchStateSet {
	var stateSet legacyRuleMatchStateSet
	for _, rule := range s.rules {
		nestedMetadata := *metadata
		nestedMetadata.ResetRuleMatchCache()
		stateSet = stateSet.merge(legacyMatchHeadlessRuleStatesWithBase(rule, &nestedMetadata, base))
	}
	return stateSet
}

func (r *abstractDefaultRule) legacyMatchStates(metadata *adapter.InboundContext) legacyRuleMatchStateSet {
	return r.legacyMatchStatesWithBase(metadata, 0)
}

func (r *abstractDefaultRule) legacyMatchStatesWithBase(metadata *adapter.InboundContext, inheritedBase ruleMatchState) legacyRuleMatchStateSet {
	if len(r.allItems) == 0 {
		return legacySingleRuleMatchState(inheritedBase)
	}
	evaluationBase := inheritedBase
	if r.invert {
		evaluationBase = 0
	}
	stateSet := legacySingleRuleMatchState(evaluationBase)
	if len(r.sourceAddressItems) > 0 {
		metadata.DidMatch = true
		if matchAnyItem(r.sourceAddressItems, metadata) {
			stateSet = stateSet.addBit(ruleMatchSourceAddress)
		}
	}
	if r.destinationIPCIDRMatchesSource(metadata) {
		metadata.DidMatch = true
		stateSet = stateSet.branchOnBit(ruleMatchSourceAddress, legacyDestinationIPFormula(r.destinationIPCIDRItems, metadata))
	}
	if len(r.sourcePortItems) > 0 {
		metadata.DidMatch = true
		if matchAnyItem(r.sourcePortItems, metadata) {
			stateSet = stateSet.addBit(ruleMatchSourcePort)
		}
	}
	if len(r.destinationAddressItems) > 0 {
		metadata.DidMatch = true
		if matchAnyItem(r.destinationAddressItems, metadata) {
			stateSet = stateSet.addBit(ruleMatchDestinationAddress)
		}
	}
	if r.legacyDestinationIPCIDRMatchesDestination(metadata) {
		metadata.DidMatch = true
		stateSet = stateSet.branchOnBit(ruleMatchDestinationAddress, legacyDestinationIPFormula(r.destinationIPCIDRItems, metadata))
	}
	if len(r.destinationPortItems) > 0 {
		metadata.DidMatch = true
		if matchAnyItem(r.destinationPortItems, metadata) {
			stateSet = stateSet.addBit(ruleMatchDestinationPort)
		}
	}
	for _, item := range r.items {
		metadata.DidMatch = true
		if !item.Match(metadata) {
			if r.invert {
				return legacySingleRuleMatchState(inheritedBase)
			}
			return legacyRuleMatchStateSet{}
		}
	}
	if r.ruleSetItem != nil {
		metadata.DidMatch = true
		var merged legacyRuleMatchStateSet
		for state := ruleMatchState(0); state < 16; state++ {
			if stateSet[state].isFalse() {
				continue
			}
			nestedStateSet := legacyMatchRuleItemStatesWithBase(r.ruleSetItem, metadata, state)
			merged = merged.merge(nestedStateSet.andFormula(stateSet[state]))
		}
		stateSet = merged
	}
	stateSet = stateSet.filter(func(state ruleMatchState) bool {
		if r.legacyRequiresSourceAddressMatch(metadata) && !state.has(ruleMatchSourceAddress) {
			return false
		}
		if len(r.sourcePortItems) > 0 && !state.has(ruleMatchSourcePort) {
			return false
		}
		if r.legacyRequiresDestinationAddressMatch(metadata) && !state.has(ruleMatchDestinationAddress) {
			return false
		}
		if len(r.destinationPortItems) > 0 && !state.has(ruleMatchDestinationPort) {
			return false
		}
		return true
	})
	if r.invert {
		return legacySingleRuleMatchStateWithFormula(inheritedBase, stateSet.anyFormula().not())
	}
	return stateSet
}

func (r *abstractDefaultRule) legacyRequiresSourceAddressMatch(metadata *adapter.InboundContext) bool {
	return len(r.sourceAddressItems) > 0 || r.destinationIPCIDRMatchesSource(metadata)
}

func (r *abstractDefaultRule) legacyDestinationIPCIDRMatchesDestination(metadata *adapter.InboundContext) bool {
	return !metadata.IPCIDRMatchSource && len(r.destinationIPCIDRItems) > 0
}

func (r *abstractDefaultRule) legacyRequiresDestinationAddressMatch(metadata *adapter.InboundContext) bool {
	return len(r.destinationAddressItems) > 0 || r.legacyDestinationIPCIDRMatchesDestination(metadata)
}

func (r *abstractLogicalRule) legacyMatchStates(metadata *adapter.InboundContext) legacyRuleMatchStateSet {
	return r.legacyMatchStatesWithBase(metadata, 0)
}

func (r *abstractLogicalRule) legacyMatchStatesWithBase(metadata *adapter.InboundContext, base ruleMatchState) legacyRuleMatchStateSet {
	evaluationBase := base
	if r.invert {
		evaluationBase = 0
	}
	var stateSet legacyRuleMatchStateSet
	if r.mode == C.LogicalTypeAnd {
		stateSet = legacySingleRuleMatchState(evaluationBase)
		for _, rule := range r.rules {
			nestedMetadata := *metadata
			nestedMetadata.ResetRuleCache()
			stateSet = stateSet.combine(legacyMatchHeadlessRuleStatesWithBase(rule, &nestedMetadata, evaluationBase))
			if stateSet.isEmpty() && !r.invert {
				return legacyRuleMatchStateSet{}
			}
		}
	} else {
		for _, rule := range r.rules {
			nestedMetadata := *metadata
			nestedMetadata.ResetRuleCache()
			stateSet = stateSet.merge(legacyMatchHeadlessRuleStatesWithBase(rule, &nestedMetadata, evaluationBase))
		}
	}
	if r.invert {
		return legacySingleRuleMatchStateWithFormula(base, stateSet.anyFormula().not())
	}
	return stateSet
}

func legacyDestinationIPFormula(items []RuleItem, metadata *adapter.InboundContext) legacyResponseFormula {
	if legacyDestinationIPResolved(metadata) {
		if matchAnyItem(items, metadata) {
			return legacyTrueFormula()
		}
		return legacyFalseFormula()
	}
	var formula legacyResponseFormula
	for _, rawItem := range items {
		switch item := rawItem.(type) {
		case *IPCIDRItem:
			if item.isSource || metadata.IPCIDRMatchSource {
				if item.Match(metadata) {
					return legacyTrueFormula()
				}
				continue
			}
			formula = formula.or(legacyLiteralFormula(legacyResponseLiteral{
				kind:  legacyLiteralRequireSet,
				ipSet: item.ipSet,
			}))
			if metadata.IPCIDRAcceptEmpty {
				formula = formula.or(legacyLiteralFormula(legacyResponseLiteral{
					kind: legacyLiteralRequireEmpty,
				}))
			}
		case *IPIsPrivateItem:
			if item.isSource {
				if item.Match(metadata) {
					return legacyTrueFormula()
				}
				continue
			}
			formula = formula.or(legacyLiteralFormula(legacyResponseLiteral{
				kind:  legacyLiteralRequireSet,
				ipSet: legacyNonPublicIPSet,
			}))
		case *IPAcceptAnyItem:
			formula = formula.or(legacyLiteralFormula(legacyResponseLiteral{
				kind: legacyLiteralRequireNonEmpty,
			}))
		default:
			if rawItem.Match(metadata) {
				return legacyTrueFormula()
			}
		}
	}
	return formula
}

func legacyDestinationIPResolved(metadata *adapter.InboundContext) bool {
	return metadata.IPCIDRMatchSource ||
		metadata.DestinationAddressMatchFromResponse ||
		metadata.DNSResponse != nil ||
		metadata.Destination.IsIP() ||
		len(metadata.DestinationAddresses) > 0
}
