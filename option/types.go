package option

import (
	"strings"

	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
	N "github.com/sagernet/sing/common/network"

	mDNS "github.com/miekg/dns"
)

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
		case N.NetworkTCP, N.NetworkUDP:
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
		return []string{N.NetworkTCP, N.NetworkUDP}
	}
	return strings.Split(string(v), "\n")
}

type DomainStrategy C.DomainStrategy

func (s DomainStrategy) String() string {
	switch C.DomainStrategy(s) {
	case C.DomainStrategyAsIS:
		return ""
	case C.DomainStrategyPreferIPv4:
		return "prefer_ipv4"
	case C.DomainStrategyPreferIPv6:
		return "prefer_ipv6"
	case C.DomainStrategyIPv4Only:
		return "ipv4_only"
	case C.DomainStrategyIPv6Only:
		return "ipv6_only"
	default:
		panic(E.New("unknown domain strategy: ", s))
	}
}

func (s DomainStrategy) MarshalJSON() ([]byte, error) {
	var value string
	switch C.DomainStrategy(s) {
	case C.DomainStrategyAsIS:
		value = ""
		// value = "as_is"
	case C.DomainStrategyPreferIPv4:
		value = "prefer_ipv4"
	case C.DomainStrategyPreferIPv6:
		value = "prefer_ipv6"
	case C.DomainStrategyIPv4Only:
		value = "ipv4_only"
	case C.DomainStrategyIPv6Only:
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
	case "", "as_is":
		*s = DomainStrategy(C.DomainStrategyAsIS)
	case "prefer_ipv4":
		*s = DomainStrategy(C.DomainStrategyPreferIPv4)
	case "prefer_ipv6":
		*s = DomainStrategy(C.DomainStrategyPreferIPv6)
	case "ipv4_only":
		*s = DomainStrategy(C.DomainStrategyIPv4Only)
	case "ipv6_only":
		*s = DomainStrategy(C.DomainStrategyIPv6Only)
	default:
		return E.New("unknown domain strategy: ", value)
	}
	return nil
}

type DNSQueryType uint16

func (t DNSQueryType) String() string {
	typeName, loaded := mDNS.TypeToString[uint16(t)]
	if loaded {
		return typeName
	}
	return F.ToString(uint16(t))
}

func (t DNSQueryType) MarshalJSON() ([]byte, error) {
	typeName, loaded := mDNS.TypeToString[uint16(t)]
	if loaded {
		return json.Marshal(typeName)
	}
	return json.Marshal(uint16(t))
}

func (t *DNSQueryType) UnmarshalJSON(bytes []byte) error {
	var valueNumber uint16
	err := json.Unmarshal(bytes, &valueNumber)
	if err == nil {
		*t = DNSQueryType(valueNumber)
		return nil
	}
	var valueString string
	err = json.Unmarshal(bytes, &valueString)
	if err == nil {
		queryType, loaded := mDNS.StringToType[valueString]
		if loaded {
			*t = DNSQueryType(queryType)
			return nil
		}
	}
	return E.New("unknown DNS query type: ", string(bytes))
}

func DNSQueryTypeToString(queryType uint16) string {
	typeName, loaded := mDNS.TypeToString[queryType]
	if loaded {
		return typeName
	}
	return F.ToString(queryType)
}

type NetworkStrategy C.NetworkStrategy

func (n NetworkStrategy) MarshalJSON() ([]byte, error) {
	return json.Marshal(C.NetworkStrategy(n).String())
}

func (n *NetworkStrategy) UnmarshalJSON(content []byte) error {
	var value string
	err := json.Unmarshal(content, &value)
	if err != nil {
		return err
	}
	strategy, loaded := C.StringToNetworkStrategy[value]
	if !loaded {
		return E.New("unknown network strategy: ", value)
	}
	*n = NetworkStrategy(strategy)
	return nil
}

type InterfaceType C.InterfaceType

func (t InterfaceType) Build() C.InterfaceType {
	return C.InterfaceType(t)
}

func (t InterfaceType) MarshalJSON() ([]byte, error) {
	return json.Marshal(C.InterfaceType(t).String())
}

func (t *InterfaceType) UnmarshalJSON(content []byte) error {
	var value string
	err := json.Unmarshal(content, &value)
	if err != nil {
		return err
	}
	interfaceType, loaded := C.StringToInterfaceType[value]
	if !loaded {
		return E.New("unknown interface type: ", value)
	}
	*t = InterfaceType(interfaceType)
	return nil
}
