package route

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
)

var _ RuleItem = (*ProcessPathPrefixItem)(nil)

type ProcessPathPrefixItem struct {
	processes []string
}

func NewProcessPathPrefixItem(directories []string) *ProcessPathPrefixItem {
	return &ProcessPathPrefixItem{
		processes: directories,
	}
}

func (r *ProcessPathPrefixItem) Match(metadata *adapter.InboundContext) bool {
	if metadata.ProcessInfo == nil || metadata.ProcessInfo.ProcessPath == "" {
		return false
	}
	for _, processe := range r.processes {
		if strings.HasPrefix(metadata.ProcessInfo.ProcessPath, processe) {
			return true
		}
	}
	return false
}

func (r *ProcessPathPrefixItem) String() string {
	var description string
	pLen := len(r.processes)
	if pLen == 1 {
		description = "process_path_prefix=" + r.processes[0]
	} else {
		description = "process_path_prefix=[" + strings.Join(r.processes, " ") + "]"
	}
	return description
}
