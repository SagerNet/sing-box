package option

import (
	"net/netip"
	"strings"

	"github.com/goccy/go-json"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
)

type ListenAddress netip.Addr

func (a ListenAddress) MarshalJSON() ([]byte, error) {
	addr := netip.Addr(a)
	if !addr.IsValid() {
		return json.Marshal("")
	}
	return json.Marshal(addr.String())
}

func (a *ListenAddress) UnmarshalJSON(content []byte) error {
	var value string
	err := json.Unmarshal(content, &value)
	if err != nil {
		return err
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return err
	}
	*a = ListenAddress(addr)
	return nil
}

type NetworkList string

func (v *NetworkList) UnmarshalJSON(content []byte) error {
	var networkList []string
	err := json.Unmarshal(content, &networkList)
	if err != nil {
		var networkItem string
		err = json.Unmarshal(content, &networkItem)
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
	*v = NetworkList(strings.Join(networkList, "\n"))
	return nil
}

func (v NetworkList) Build() []string {
	if v == "" {
		return []string{C.NetworkTCP, C.NetworkUDP}
	}
	return strings.Split(string(v), "\n")
}

type Listable[T comparable] []T

func (l Listable[T]) MarshalJSON() ([]byte, error) {
	arrayList := []T(l)
	if len(arrayList) == 1 {
		return json.Marshal(arrayList[0])
	}
	return json.Marshal(arrayList)
}

func (l *Listable[T]) UnmarshalJSON(content []byte) error {
	err := json.Unmarshal(content, (*[]T)(l))
	if err == nil {
		return nil
	}
	var singleItem T
	err = json.Unmarshal(content, &singleItem)
	if err != nil {
		return err
	}
	*l = []T{singleItem}
	return nil
}
