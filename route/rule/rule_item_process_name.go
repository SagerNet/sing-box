package rule

import (
	"path/filepath"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
)

var _ RuleItem = (*ProcessItem)(nil)

type ProcessItem struct {
	processes  []string
	processMap map[string]bool
}

func NewProcessItem(processNameList []string) *ProcessItem {
	rule := &ProcessItem{
		processes:  processNameList,
		processMap: make(map[string]bool),
	}
	for _, processName := range processNameList {
		if C.IsWindows {
			processName = strings.ToLower(processName)
		}
		rule.processMap[processName] = true
	}
	return rule
}

func (r *ProcessItem) Match(metadata *adapter.InboundContext) bool {
	if metadata.ProcessInfo == nil || metadata.ProcessInfo.ProcessPath == "" {
		return false
	}
	path := filepath.Base(metadata.ProcessInfo.ProcessPath)
	if C.IsWindows {
		path = strings.ToLower(path)
	}
	return r.processMap[path]
}

func (r *ProcessItem) String() string {
	var description string
	pLen := len(r.processes)
	if pLen == 1 {
		description = "process_name=" + r.processes[0]
	} else {
		description = "process_name=[" + strings.Join(r.processes, " ") + "]"
	}
	return description
}
