package option

import (
	"net/netip"
	"strconv"

	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
)

type TunInboundOptions struct {
	InterfaceName          string                 `json:"interface_name,omitempty"`
	MTU                    uint32                 `json:"mtu,omitempty"`
	GSO                    bool                   `json:"gso,omitempty"`
	Address                Listable[netip.Prefix] `json:"address,omitempty"`
	AutoRoute              bool                   `json:"auto_route,omitempty"`
	IPRoute2TableIndex     int                    `json:"iproute2_table_index,omitempty"`
	IPRoute2RuleIndex      int                    `json:"iproute2_rule_index,omitempty"`
	AutoRedirect           bool                   `json:"auto_redirect,omitempty"`
	AutoRedirectInputMark  FwMark                 `json:"auto_redirect_input_mark,omitempty"`
	AutoRedirectOutputMark FwMark                 `json:"auto_redirect_output_mark,omitempty"`
	StrictRoute            bool                   `json:"strict_route,omitempty"`
	RouteAddress           Listable[netip.Prefix] `json:"route_address,omitempty"`
	RouteAddressSet        Listable[string]       `json:"route_address_set,omitempty"`
	RouteExcludeAddress    Listable[netip.Prefix] `json:"route_exclude_address,omitempty"`
	RouteExcludeAddressSet Listable[string]       `json:"route_exclude_address_set,omitempty"`
	IncludeInterface       Listable[string]       `json:"include_interface,omitempty"`
	ExcludeInterface       Listable[string]       `json:"exclude_interface,omitempty"`
	IncludeUID             Listable[uint32]       `json:"include_uid,omitempty"`
	IncludeUIDRange        Listable[string]       `json:"include_uid_range,omitempty"`
	ExcludeUID             Listable[uint32]       `json:"exclude_uid,omitempty"`
	ExcludeUIDRange        Listable[string]       `json:"exclude_uid_range,omitempty"`
	IncludeAndroidUser     Listable[int]          `json:"include_android_user,omitempty"`
	IncludePackage         Listable[string]       `json:"include_package,omitempty"`
	ExcludePackage         Listable[string]       `json:"exclude_package,omitempty"`
	EndpointIndependentNat bool                   `json:"endpoint_independent_nat,omitempty"`
	UDPTimeout             UDPTimeoutCompat       `json:"udp_timeout,omitempty"`
	Stack                  string                 `json:"stack,omitempty"`
	Platform               *TunPlatformOptions    `json:"platform,omitempty"`
	InboundOptions

	// Deprecated: merged to Address
	Inet4Address Listable[netip.Prefix] `json:"inet4_address,omitempty"`
	// Deprecated: merged to Address
	Inet6Address Listable[netip.Prefix] `json:"inet6_address,omitempty"`
	// Deprecated: merged to RouteAddress
	Inet4RouteAddress Listable[netip.Prefix] `json:"inet4_route_address,omitempty"`
	// Deprecated: merged to RouteAddress
	Inet6RouteAddress Listable[netip.Prefix] `json:"inet6_route_address,omitempty"`
	// Deprecated: merged to RouteExcludeAddress
	Inet4RouteExcludeAddress Listable[netip.Prefix] `json:"inet4_route_exclude_address,omitempty"`
	// Deprecated: merged to RouteExcludeAddress
	Inet6RouteExcludeAddress Listable[netip.Prefix] `json:"inet6_route_exclude_address,omitempty"`
}

type FwMark uint32

func (f FwMark) MarshalJSON() ([]byte, error) {
	return json.Marshal(F.ToString("0x", strconv.FormatUint(uint64(f), 16)))
}

func (f *FwMark) UnmarshalJSON(bytes []byte) error {
	var stringValue string
	err := json.Unmarshal(bytes, &stringValue)
	if err != nil {
		if rawErr := json.Unmarshal(bytes, (*uint32)(f)); rawErr == nil {
			return nil
		}
		return E.Cause(err, "invalid number or string mark")
	}
	intValue, err := strconv.ParseUint(stringValue, 0, 32)
	if err != nil {
		return err
	}
	*f = FwMark(intValue)
	return nil
}
