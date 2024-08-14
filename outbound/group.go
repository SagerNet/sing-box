package outbound

import (
	"context"
	R "github.com/dlclark/regexp2"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
)

type myGroupAdapter struct {
	ctx             context.Context
	tags            []string
	uses            []string
	useAllProviders bool
	includes        []*R.Regexp
	excludes        *R.Regexp
	types           []string
	providers       map[string]adapter.OutboundProvider
}

func CheckType(types []string) bool {
	return common.All(types, func(it string) bool {
		switch it {
		case C.TypeTor, C.TypeSSH, C.TypeHTTP, C.TypeSOCKS, C.TypeTUIC, C.TypeVMess, C.TypeVLESS, C.TypeTrojan, C.TypeShadowTLS, C.TypeShadowsocks, C.TypeShadowsocksR, C.TypeHysteria, C.TypeHysteria2, C.TypeWireGuard:
			return true
		}
		return false
	})
}

func (s *myGroupAdapter) OutboundFilter(out adapter.Outbound) bool {
	return TestIncludes(out.Tag(), s.includes) && TestExcludes(out.Tag(), s.excludes) && TestTypes(out.Type(), s.types)
}

func TestIncludes(tag string, includes []*R.Regexp) bool {
	if len(includes) == 0 {
		return true
	}
	return common.All(includes, func(it *R.Regexp) bool {
		matched, _ := it.MatchString(tag)
		return matched
	})
}

func TestExcludes(tag string, excludes *R.Regexp) bool {
	if excludes == nil {
		return true
	}
	matched, _ := excludes.MatchString(tag)
	return !matched
}

func TestTypes(oType string, types []string) bool {
	if len(types) == 0 {
		return true
	}
	return common.Any(types, func(it string) bool {
		return oType == it
	})
}
