package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*ProtocolItem)(nil)

type ProtocolItem struct {
	protocols   []string
	protocolMap map[string]bool
}

func NewProtocolItem(protocols []string) *ProtocolItem {
	protocolMap := make(map[string]bool)
	for _, protocol := range protocols {
		protocolMap[protocol] = true
	}
	return &ProtocolItem{
		protocols:   protocols,
		protocolMap: protocolMap,
	}
}

func (r *ProtocolItem) Match(metadata *adapter.InboundContext) bool {
	return r.protocolMap[metadata.Protocol]
}

func (r *ProtocolItem) String() string {
	if len(r.protocols) == 1 {
		return F.ToString("protocol=", r.protocols[0])
	}
	return F.ToString("protocol=[", strings.Join(r.protocols, " "), "]")
}
