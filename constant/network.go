package constant

import (
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
)

const (
	InterfaceTypeWIFI     = "wifi"
	InterfaceTypeCellular = "cellular"
	InterfaceTypeEthernet = "ethernet"
	InterfaceTypeOther    = "other"
)

type NetworkStrategy int

const (
	NetworkStrategyDefault NetworkStrategy = iota
	NetworkStrategyFallback
	NetworkStrategyHybrid
	NetworkStrategyWIFI
	NetworkStrategyCellular
	NetworkStrategyEthernet
	NetworkStrategyWIFIOnly
	NetworkStrategyCellularOnly
	NetworkStrategyEthernetOnly
)

var (
	NetworkStrategyToString = map[NetworkStrategy]string{
		NetworkStrategyDefault:      "default",
		NetworkStrategyFallback:     "fallback",
		NetworkStrategyHybrid:       "hybrid",
		NetworkStrategyWIFI:         "wifi",
		NetworkStrategyCellular:     "cellular",
		NetworkStrategyEthernet:     "ethernet",
		NetworkStrategyWIFIOnly:     "wifi_only",
		NetworkStrategyCellularOnly: "cellular_only",
		NetworkStrategyEthernetOnly: "ethernet_only",
	}
	StringToNetworkStrategy = common.ReverseMap(NetworkStrategyToString)
)

func (s NetworkStrategy) String() string {
	name, loaded := NetworkStrategyToString[s]
	if !loaded {
		return F.ToString(int(s))
	}
	return name
}
