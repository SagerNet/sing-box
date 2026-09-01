//nolint:unused
package libbox

import (
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/ranges"
)

type autoRedirectOptions struct {
	InterfaceName            string                 `json:"interface_name"`
	Inet4Address             []netip.Prefix         `json:"inet4_address,omitempty"`
	Inet6Address             []netip.Prefix         `json:"inet6_address,omitempty"`
	MTU                      uint32                 `json:"mtu,omitempty"`
	DNSMode                  string                 `json:"dns_mode,omitempty"`
	DNSAddress               []netip.Addr           `json:"dns_address,omitempty"`
	IPRoute2TableIndex       int                    `json:"iproute2_table_index,omitempty"`
	IPRoute2RuleIndex        int                    `json:"iproute2_rule_index,omitempty"`
	AutoRedirectInputMark    uint32                 `json:"auto_redirect_input_mark,omitempty"`
	AutoRedirectOutputMark   uint32                 `json:"auto_redirect_output_mark,omitempty"`
	AutoRedirectResetMark    uint32                 `json:"auto_redirect_reset_mark,omitempty"`
	AutoRedirectTProxyMark   uint32                 `json:"auto_redirect_tproxy_mark,omitempty"`
	AutoRedirectNFQueue      uint16                 `json:"auto_redirect_nfqueue,omitempty"`
	ExcludeMPTCP             bool                   `json:"exclude_mptcp,omitempty"`
	Inet4LoopbackAddress     []netip.Addr           `json:"inet4_loopback_address,omitempty"`
	Inet6LoopbackAddress     []netip.Addr           `json:"inet6_loopback_address,omitempty"`
	StrictRoute              bool                   `json:"strict_route,omitempty"`
	Inet4RouteAddress        []netip.Prefix         `json:"inet4_route_address,omitempty"`
	Inet6RouteAddress        []netip.Prefix         `json:"inet6_route_address,omitempty"`
	Inet4RouteExcludeAddress []netip.Prefix         `json:"inet4_route_exclude_address,omitempty"`
	Inet6RouteExcludeAddress []netip.Prefix         `json:"inet6_route_exclude_address,omitempty"`
	IncludeInterface         []string               `json:"include_interface,omitempty"`
	ExcludeInterface         []string               `json:"exclude_interface,omitempty"`
	IncludeUID               []ranges.Range[uint32] `json:"include_uid,omitempty"`
	ExcludeUID               []ranges.Range[uint32] `json:"exclude_uid,omitempty"`
	IncludeMACAddress        []net.HardwareAddr     `json:"include_mac_address,omitempty"`
	ExcludeMACAddress        []net.HardwareAddr     `json:"exclude_mac_address,omitempty"`
	TableName                string                 `json:"table_name"`
	RedirectPort             uint16                 `json:"redirect_port"`
}

func encodeAutoRedirectOptions(options adapter.AutoRedirectOptions) ([]byte, error) {
	tunOptions := options.TunOptions
	content, err := json.Marshal(autoRedirectOptions{
		InterfaceName:            tunOptions.Name,
		Inet4Address:             tunOptions.Inet4Address,
		Inet6Address:             tunOptions.Inet6Address,
		MTU:                      tunOptions.MTU,
		DNSMode:                  tunOptions.DNSMode,
		DNSAddress:               tunOptions.DNSAddress,
		IPRoute2TableIndex:       tunOptions.IPRoute2TableIndex,
		IPRoute2RuleIndex:        tunOptions.IPRoute2RuleIndex,
		AutoRedirectInputMark:    tunOptions.AutoRedirectInputMark,
		AutoRedirectOutputMark:   tunOptions.AutoRedirectOutputMark,
		AutoRedirectResetMark:    tunOptions.AutoRedirectResetMark,
		AutoRedirectTProxyMark:   tunOptions.AutoRedirectTProxyMark,
		AutoRedirectNFQueue:      tunOptions.AutoRedirectNFQueue,
		ExcludeMPTCP:             tunOptions.ExcludeMPTCP,
		Inet4LoopbackAddress:     tunOptions.Inet4LoopbackAddress,
		Inet6LoopbackAddress:     tunOptions.Inet6LoopbackAddress,
		StrictRoute:              tunOptions.StrictRoute,
		Inet4RouteAddress:        tunOptions.Inet4RouteAddress,
		Inet6RouteAddress:        tunOptions.Inet6RouteAddress,
		Inet4RouteExcludeAddress: tunOptions.Inet4RouteExcludeAddress,
		Inet6RouteExcludeAddress: tunOptions.Inet6RouteExcludeAddress,
		IncludeInterface:         tunOptions.IncludeInterface,
		ExcludeInterface:         tunOptions.ExcludeInterface,
		IncludeUID:               tunOptions.IncludeUID,
		ExcludeUID:               tunOptions.ExcludeUID,
		IncludeMACAddress:        tunOptions.IncludeMACAddress,
		ExcludeMACAddress:        tunOptions.ExcludeMACAddress,
		TableName:                options.TableName,
		RedirectPort:             options.RedirectPort,
	})
	if err != nil {
		return nil, E.Cause(err, "encode auto-redirect options")
	}
	return content, nil
}

func decodeAutoRedirectOptions(content []byte) (*tun.Options, string, uint16, error) {
	var options autoRedirectOptions
	err := json.Unmarshal(content, &options)
	if err != nil {
		return nil, "", 0, E.Cause(err, "decode auto-redirect options")
	}
	return &tun.Options{
		Name:                     options.InterfaceName,
		Inet4Address:             options.Inet4Address,
		Inet6Address:             options.Inet6Address,
		MTU:                      options.MTU,
		DNSMode:                  options.DNSMode,
		DNSAddress:               options.DNSAddress,
		IPRoute2TableIndex:       options.IPRoute2TableIndex,
		IPRoute2RuleIndex:        options.IPRoute2RuleIndex,
		AutoRedirectInputMark:    options.AutoRedirectInputMark,
		AutoRedirectOutputMark:   options.AutoRedirectOutputMark,
		AutoRedirectResetMark:    options.AutoRedirectResetMark,
		AutoRedirectTProxyMark:   options.AutoRedirectTProxyMark,
		AutoRedirectNFQueue:      options.AutoRedirectNFQueue,
		ExcludeMPTCP:             options.ExcludeMPTCP,
		Inet4LoopbackAddress:     options.Inet4LoopbackAddress,
		Inet6LoopbackAddress:     options.Inet6LoopbackAddress,
		StrictRoute:              options.StrictRoute,
		Inet4RouteAddress:        options.Inet4RouteAddress,
		Inet6RouteAddress:        options.Inet6RouteAddress,
		Inet4RouteExcludeAddress: options.Inet4RouteExcludeAddress,
		Inet6RouteExcludeAddress: options.Inet6RouteExcludeAddress,
		IncludeInterface:         options.IncludeInterface,
		ExcludeInterface:         options.ExcludeInterface,
		IncludeUID:               options.IncludeUID,
		ExcludeUID:               options.ExcludeUID,
		IncludeMACAddress:        options.IncludeMACAddress,
		ExcludeMACAddress:        options.ExcludeMACAddress,
	}, options.TableName, options.RedirectPort, nil
}
