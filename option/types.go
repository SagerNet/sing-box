package option

import (
	"strings"

	"github.com/sagernet/sing-dns"
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

type DomainStrategy dns.DomainStrategy

func (s DomainStrategy) String() string {
	switch dns.DomainStrategy(s) {
	case dns.DomainStrategyAsIS:
		return ""
	case dns.DomainStrategyPreferIPv4:
		return "prefer_ipv4"
	case dns.DomainStrategyPreferIPv6:
		return "prefer_ipv6"
	case dns.DomainStrategyUseIPv4:
		return "ipv4_only"
	case dns.DomainStrategyUseIPv6:
		return "ipv6_only"
	default:
		panic(E.New("unknown domain strategy: ", s))
	}
}

func (s DomainStrategy) MarshalJSON() ([]byte, error) {
	var value string
	switch dns.DomainStrategy(s) {
	case dns.DomainStrategyAsIS:
		value = ""
		// value = "as_is"
	case dns.DomainStrategyPreferIPv4:
		value = "prefer_ipv4"
	case dns.DomainStrategyPreferIPv6:
		value = "prefer_ipv6"
	case dns.DomainStrategyUseIPv4:
		value = "ipv4_only"
	case dns.DomainStrategyUseIPv6:
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
		*s = DomainStrategy(dns.DomainStrategyAsIS)
	case "prefer_ipv4":
		*s = DomainStrategy(dns.DomainStrategyPreferIPv4)
	case "prefer_ipv6":
		*s = DomainStrategy(dns.DomainStrategyPreferIPv6)
	case "ipv4_only":
		*s = DomainStrategy(dns.DomainStrategyUseIPv4)
	case "ipv6_only":
		*s = DomainStrategy(dns.DomainStrategyUseIPv6)
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
