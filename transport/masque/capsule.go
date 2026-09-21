package masque

import (
	"net/netip"

	transportHTTP "github.com/sagernet/sing-box/transport/http"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"

	"go4.org/netipx"
)

const (
	capsuleTypeAddressAssign      = 0x01
	capsuleTypeAddressRequest     = 0x02
	capsuleTypeRouteAdvertisement = 0x03
)

type AssignedAddress struct {
	RequestID uint64
	Prefix    netip.Prefix
}

type AddressRange struct {
	Start    netip.Addr
	End      netip.Addr
	Protocol uint8
}

func (r AddressRange) Contains(address netip.Addr) bool {
	return address.BitLen() == r.Start.BitLen() && r.Start.Compare(address) <= 0 && address.Compare(r.End) <= 0
}

func parseAddresses(payload []byte) ([]AssignedAddress, error) {
	var addresses []AssignedAddress
	for len(payload) > 0 {
		requestID, requestIDLength, valid := transportHTTP.DecodeVarint(payload)
		if !valid {
			return nil, E.New("truncated request ID")
		}
		payload = payload[requestIDLength:]
		addressLength, err := parseVersion(payload)
		if err != nil {
			return nil, err
		}
		if len(payload) < 1+addressLength+1 {
			return nil, E.New("truncated address")
		}
		address, _ := netip.AddrFromSlice(payload[1 : 1+addressLength])
		prefixLength := int(payload[1+addressLength])
		payload = payload[1+addressLength+1:]
		if prefixLength > address.BitLen() {
			return nil, E.New("invalid prefix length: ", prefixLength)
		}
		prefix := netip.PrefixFrom(address, prefixLength)
		if prefix.Masked() != prefix {
			return nil, E.New("prefix has host bits set: ", prefix)
		}
		addresses = append(addresses, AssignedAddress{RequestID: requestID, Prefix: prefix})
	}
	return addresses, nil
}

func parseVersion(payload []byte) (int, error) {
	if len(payload) < 1 {
		return 0, E.New("truncated IP version")
	}
	switch payload[0] {
	case 4:
		return 4, nil
	case 6:
		return 16, nil
	default:
		return 0, E.New("invalid IP version: ", payload[0])
	}
}

func parseRoutes(payload []byte) ([]AddressRange, error) {
	var routes []AddressRange
	for len(payload) > 0 {
		addressLength, err := parseVersion(payload)
		if err != nil {
			return nil, err
		}
		if len(payload) < 1+addressLength*2+1 {
			return nil, E.New("truncated address range")
		}
		start, _ := netip.AddrFromSlice(payload[1 : 1+addressLength])
		end, _ := netip.AddrFromSlice(payload[1+addressLength : 1+addressLength*2])
		route := AddressRange{Start: start, End: end, Protocol: payload[1+addressLength*2]}
		payload = payload[1+addressLength*2+1:]
		if start.Compare(end) > 0 {
			return nil, E.New("invalid address range: ", start, "-", end)
		}
		if len(routes) > 0 {
			previous := routes[len(routes)-1]
			switch {
			case previous.Start.BitLen() > start.BitLen():
				return nil, E.New("address ranges are not ordered by IP version")
			case previous.Start.BitLen() < start.BitLen():
			case previous.Protocol > route.Protocol:
				return nil, E.New("address ranges are not ordered by IP protocol")
			case previous.Protocol < route.Protocol:
			case previous.End.Compare(start) >= 0:
				return nil, E.New("address ranges overlap: ", previous.End, " and ", start)
			}
		}
		routes = append(routes, route)
	}
	return routes, nil
}

func appendAddress(payload []byte, address netip.Addr) []byte {
	if address.Is4() {
		payload = append(payload, 4)
	} else {
		payload = append(payload, 6)
	}
	return append(payload, address.AsSlice()...)
}

func newCapsule(capsuleType uint64, payload []byte) *buf.Buffer {
	capsule := buf.NewSize(transportHTTP.VarintLen(capsuleType) + transportHTTP.VarintLen(uint64(len(payload))) + len(payload))
	transportHTTP.PutVarint(capsule.Extend(transportHTTP.VarintLen(capsuleType)), capsuleType)
	transportHTTP.PutVarint(capsule.Extend(transportHTTP.VarintLen(uint64(len(payload)))), uint64(len(payload)))
	capsule.Write(payload)
	return capsule
}

func newAddressCapsule(capsuleType uint64, addresses []AssignedAddress) *buf.Buffer {
	var payload []byte
	for _, address := range addresses {
		var requestID [8]byte
		payload = append(payload, requestID[:transportHTTP.PutVarint(requestID[:], address.RequestID)]...)
		payload = appendAddress(payload, address.Prefix.Addr())
		payload = append(payload, byte(address.Prefix.Bits()))
	}
	return newCapsule(capsuleType, payload)
}

func newRouteCapsule(routes []AddressRange) *buf.Buffer {
	var payload []byte
	for _, route := range routes {
		payload = appendAddress(payload, route.Start)
		payload = append(payload, route.End.AsSlice()...)
		payload = append(payload, route.Protocol)
	}
	return newCapsule(capsuleTypeRouteAdvertisement, payload)
}

func RangesFromPrefixes(prefixes []netip.Prefix, protocol uint8) ([]AddressRange, error) {
	var builder netipx.IPSetBuilder
	for _, prefix := range prefixes {
		builder.AddPrefix(prefix)
	}
	ipSet, err := builder.IPSet()
	if err != nil {
		return nil, err
	}
	ipRanges := ipSet.Ranges()
	routes := make([]AddressRange, 0, len(ipRanges))
	for _, ipRange := range ipRanges {
		routes = append(routes, AddressRange{Start: ipRange.From(), End: ipRange.To(), Protocol: protocol})
	}
	return routes, nil
}
