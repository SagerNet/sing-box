package option

type BridgeOutboundOptions struct {
	Interface          string `json:"interface,omitempty"`
	BridgeName         string `json:"bridge_name,omitempty"`
	IPRoute2TableIndex int    `json:"iproute2_table_index,omitempty"`
	IPRoute2RuleIndex  int    `json:"iproute2_rule_index,omitempty"`
}
