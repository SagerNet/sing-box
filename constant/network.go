package constant

import (
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
)

type InterfaceType uint8

const (
	InterfaceTypeWIFI InterfaceType = iota
	InterfaceTypeCellular
	InterfaceTypeEthernet
	InterfaceTypeOther
)

var (
	interfaceTypeToString = map[InterfaceType]string{
		InterfaceTypeWIFI:     "wifi",
		InterfaceTypeCellular: "cellular",
		InterfaceTypeEthernet: "ethernet",
		InterfaceTypeOther:    "other",
	}
	StringToInterfaceType = common.ReverseMap(interfaceTypeToString)
)

func (t InterfaceType) String() string {
	name, loaded := interfaceTypeToString[t]
	if !loaded {
		return F.ToString(int(t))
	}
	return name
}

type NetworkStrategy uint8

const (
	NetworkStrategyDefault NetworkStrategy = iota
	NetworkStrategyFallback
	NetworkStrategyHybrid
)

var (
	networkStrategyToString = map[NetworkStrategy]string{
		NetworkStrategyDefault:  "default",
		NetworkStrategyFallback: "fallback",
		NetworkStrategyHybrid:   "hybrid",
	}
	StringToNetworkStrategy = common.ReverseMap(networkStrategyToString)
)

func (s NetworkStrategy) String() string {
	name, loaded := networkStrategyToString[s]
	if !loaded {
		return F.ToString(int(s))
	}
	return name
}
