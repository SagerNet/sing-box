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
	RuleSetVersionCurrent = RuleSetVersion2
)

const (
	RuleActionTypeRoute     = "route"
	RuleActionTypeReturn    = "return"
	RuleActionTypeReject    = "reject"
	RuleActionTypeHijackDNS = "hijack-dns"
	RuleActionTypeSniff     = "sniff"
	RuleActionTypeResolve   = "resolve"
)

const (
	RuleActionRejectMethodDefault            = "default"
	RuleActionRejectMethodReset              = "reset"
	RuleActionRejectMethodNetworkUnreachable = "network-unreachable"
	RuleActionRejectMethodHostUnreachable    = "host-unreachable"
	RuleActionRejectMethodPortUnreachable    = "port-unreachable"
	RuleActionRejectMethodDrop               = "drop"
)
