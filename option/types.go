package option

import (
	"net/netip"
	"strings"
	"time"

	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/goccy/go-json"
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

type DomainStrategy C.DomainStrategy

func (s DomainStrategy) MarshalJSON() ([]byte, error) {
	var value string
	switch C.DomainStrategy(s) {
	case C.DomainStrategyAsIS:
		value = ""
		// value = "AsIS"
	case C.DomainStrategyPreferIPv4:
		value = "prefer_ipv4"
	case C.DomainStrategyPreferIPv6:
		value = "prefer_ipv6"
	case C.DomainStrategyUseIPv4:
		value = "ipv4_only"
	case C.DomainStrategyUseIPv6:
		value = "ipv6_only"
	default:
		return nil, E.New("unknown domain strategy: ", s)
	}
	return json.Marshal(value)
}

func (s *DomainStrategy) UnmarshalJSON(bytes []byte) error {
	var value string
	err := json.Unmarshal(bytes, &value)
	if err != nil {
		return err
	}
	switch value {
	case "", "AsIS":
		*s = DomainStrategy(C.DomainStrategyAsIS)
	case "prefer_ipv4":
		*s = DomainStrategy(C.DomainStrategyPreferIPv4)
	case "prefer_ipv6":
		*s = DomainStrategy(C.DomainStrategyPreferIPv6)
	case "ipv4_only":
		*s = DomainStrategy(C.DomainStrategyUseIPv4)
	case "ipv6_only":
		*s = DomainStrategy(C.DomainStrategyUseIPv6)
	default:
		return E.New("unknown domain strategy: ", value)
	}
	return nil
}

type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal((time.Duration)(d).String())
}

func (d *Duration) UnmarshalJSON(bytes []byte) error {
	var value string
	err := json.Unmarshal(bytes, &value)
	if err != nil {
		return err
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	*d = Duration(duration)
	return nil
}

type ListenPrefix netip.Prefix

func (p ListenPrefix) MarshalJSON() ([]byte, error) {
	prefix := netip.Prefix(p)
	if !prefix.IsValid() {
		return json.Marshal("")
	}
	return json.Marshal(prefix.String())
}

func (p *ListenPrefix) UnmarshalJSON(bytes []byte) error {
	var value string
	err := json.Unmarshal(bytes, &value)
	if err != nil {
		return err
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return err
	}
	*p = ListenPrefix(prefix)
	return nil
}
