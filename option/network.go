package option

import (
	"github.com/goccy/go-json"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
)

type NetworkList []string

func (v *NetworkList) UnmarshalJSON(data []byte) error {
	var networkList []string
	err := json.Unmarshal(data, &networkList)
	if err != nil {
		var networkItem string
		err = json.Unmarshal(data, &networkItem)
		if err != nil {
			return err
		}
		networkList = []string{networkItem}
	}
	for _, networkName := range networkList {
		switch networkName {
		case C.NetworkTCP, C.NetworkUDP:
			break
		default:
			return E.New("unknown network: " + networkName)
		}
	}
	*v = networkList
	return nil
}

func (v *NetworkList) Build() []string {
	if len(*v) == 0 {
		return []string{C.NetworkTCP, C.NetworkUDP}
	}
	return *v
}
