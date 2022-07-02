package option

import (
	"encoding/json"
	"strings"

	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
)

type NetworkList []string

func (v *NetworkList) UnmarshalJSON(data []byte) error {
	var networkList string
	err := json.Unmarshal(data, &networkList)
	if err != nil {
		return err
	}
	for _, networkName := range strings.Split(networkList, ",") {
		switch networkName {
		case "tcp", "udp":
			*v = append(*v, networkName)
		default:
			return E.New("unknown network: " + networkName)
		}
	}
	return nil
}

func (v *NetworkList) Build() []string {
	if len(*v) == 0 {
		return []string{C.NetworkTCP, C.NetworkUDP}
	}
	return *v
}
