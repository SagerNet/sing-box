package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*ProcessPIDItem)(nil)

type ProcessPIDItem struct {
	processPIDs   []uint32
	processPIDMap map[uint32]bool
}

func NewProcessPIDItem(processPIDList []uint32) *ProcessPIDItem {
	rule := &ProcessPIDItem{
		processPIDs:   processPIDList,
		processPIDMap: make(map[uint32]bool),
	}
	for _, processPID := range processPIDList {
		rule.processPIDMap[processPID] = true
	}
	return rule
}

func (r *ProcessPIDItem) Match(metadata *adapter.InboundContext) bool {
	if metadata.ProcessInfo == nil || metadata.ProcessInfo.ProcessID == 0 {
		return false
	}
	return r.processPIDMap[metadata.ProcessInfo.ProcessID]
}

func (r *ProcessPIDItem) String() string {
	var description string
	pLen := len(r.processPIDs)
	if pLen == 1 {
		description = "process_pid=" + F.ToString(r.processPIDs[0])
	} else {
		description = "process_pid=[" + strings.Join(F.MapToString(r.processPIDs), " ") + "]"
	}
	return description
}
