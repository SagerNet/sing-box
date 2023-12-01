package option

import (
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-dns"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	N "github.com/sagernet/sing/common/network"

	mDNS "github.com/miekg/dns"
)

type ListenAddress netip.Addr

func NewListenAddress(addr netip.Addr) *ListenAddress {
	address := ListenAddress(addr)
	return &address
}

func (a ListenAddress) MarshalJSON() ([]byte, error) {
	addr := netip.Addr(a)
	if !addr.IsValid() {
		return nil, nil
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

func (a *ListenAddress) Build() netip.Addr {
	if a == nil {
		return netip.AddrFrom4([4]byte{127, 0, 0, 1})
	}
	return (netip.Addr)(*a)
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
	newError := json.Unmarshal(content, &singleItem)
	if newError != nil {
		return E.Errors(err, newError)
	}
	*l = []T{singleItem}
	return nil
}

type DomainStrategy dns.DomainStrategy

func (s DomainStrategy) MarshalJSON() ([]byte, error) {
	var value string
	switch dns.DomainStrategy(s) {
	case dns.DomainStrategyAsIS:
		value = ""
		// value = "AsIS"
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
	duration, err := ParseDuration(value)
	if err != nil {
		return err
	}
	*d = Duration(duration)
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

type HTTPHeader map[string]Listable[string]

func (h HTTPHeader) Build() http.Header {
	header := make(http.Header)
	for name, values := range h {
		for _, value := range values {
			header.Add(name, value)
		}
	}
	return header
}
