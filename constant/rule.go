package constant

const (
	RuleTypeDefault = "default"
	RuleTypeLogical = "logical"
)

const (
	LogicalTypeAnd = "and"
	LogicalTypeOr  = "or"
)

const (
	RuleSetTypeInline   = "inline"
	RuleSetTypeLocal    = "local"
	RuleSetTypeRemote   = "remote"
	RuleSetFormatSource = "source"
	RuleSetFormatBinary = "binary"
)

const (
	RuleSetVersion1 = 1 + iota
	RuleSetVersion2
	RuleSetVersion3
	RuleSetVersion4
	RuleSetVersion5
	RuleSetVersionCurrent = RuleSetVersion5
)

const (
	RuleActionTypeRoute        = "route"
	RuleActionTypeRouteOptions = "route-options"
	RuleActionTypeEvaluate     = "evaluate"
	RuleActionTypeRespond      = "respond"
	RuleActionTypeDirect       = "direct"
	RuleActionTypeBypass       = "bypass"
	RuleActionTypeReject       = "reject"
	RuleActionTypeHijackDNS    = "hijack-dns"
	RuleActionTypeSniff        = "sniff"
	RuleActionTypeResolve      = "resolve"
	RuleActionTypePredefined   = "predefined"
)

const (
	RuleActionRejectMethodDefault = "default"
	RuleActionRejectMethodDrop    = "drop"
	RuleActionRejectMethodReply   = "reply"
)
