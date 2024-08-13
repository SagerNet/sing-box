package outbound

import (
	"context"
	"strconv"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	R "github.com/dlclark/regexp2"
)

type myGroupAdapter struct {
	ctx             context.Context
	tags            []string
	uses            []string
	useAllProviders bool
	includes        []*R.Regexp
	excludes        *R.Regexp
	types           []string
	ports           map[int]bool
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

func CreatePortsMap(ports []string) (map[int]bool, error) {
	portReg1 := R.MustCompile(`^\d+$`, R.None)
	portReg2 := R.MustCompile(`^(\d*):(\d*)$`, R.None)
	portMap := map[int]bool{}
	for i, portRaw := range ports {
		if matched, _ := portReg1.MatchString(portRaw); matched {
			port, _ := strconv.Atoi(portRaw)
			if port < 0 || port > 65535 {
				return nil, E.New("invalid ports item[", i, "]")
			}
			portMap[port] = true
			continue
		}
		if portRaw == ":" {
			return nil, E.New("invalid ports item[", i, "]")
		}
		if match, _ := portReg2.FindStringMatch(portRaw); match != nil {
			start, _ := strconv.Atoi(match.Groups()[1].String())
			end, _ := strconv.Atoi(match.Groups()[2].String())
			if start < 0 || start > 65535 {
				return nil, E.New("invalid ports item[", i, "]")
			}
			if end < 0 || end > 65535 {
				return nil, E.New("invalid ports item[", i, "]")
			}
			if end == 0 {
				end = 65535
			}
			if start > end {
				return nil, E.New("invalid ports item[", i, "]")
			}
			for port := start; port <= end; port++ {
				portMap[port] = true
			}
			continue
		}
		return nil, E.New("invalid ports item[", i, "]")
	}
	return portMap, nil
}

func (s *myGroupAdapter) OutboundFilter(out adapter.Outbound) bool {
	return TestIncludes(out.Tag(), s.includes) && TestExcludes(out.Tag(), s.excludes) && TestTypes(out.Type(), s.types) && TestPorts(out.Port(), s.ports)
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

func TestPorts(port int, ports map[int]bool) bool {
	if port == 0 || len(ports) == 0 {
		return true
	}
	_, ok := ports[port]
	return ok
}
