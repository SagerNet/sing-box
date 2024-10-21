package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
)

var _ RuleItem = (*PackageNameItem)(nil)

type PackageNameItem struct {
	packageNames []string
	packageMap   map[string]bool
}

func NewPackageNameItem(packageNameList []string) *PackageNameItem {
	rule := &PackageNameItem{
		packageNames: packageNameList,
		packageMap:   make(map[string]bool),
	}
	for _, packageName := range packageNameList {
		rule.packageMap[packageName] = true
	}
	return rule
}

func (r *PackageNameItem) Match(metadata *adapter.InboundContext) bool {
	if metadata.ProcessInfo == nil || metadata.ProcessInfo.PackageName == "" {
		return false
	}
	return r.packageMap[metadata.ProcessInfo.PackageName]
}

func (r *PackageNameItem) String() string {
	var description string
	pLen := len(r.packageNames)
	if pLen == 1 {
		description = "package_name=" + r.packageNames[0]
	} else {
		description = "package_name=[" + strings.Join(r.packageNames, " ") + "]"
	}
	return description
}
