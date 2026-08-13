package main

import (
	"net/netip"
	"slices"
	"strings"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandAPITailscalePeer = &cobra.Command{
	Use:   "peer",
	Short: "Manage Tailscale peers",
}

func init() {
	commandAPITailscalePeer.PersistentFlags().StringVar(&commandAPITailscaleFlagEndpoint, "endpoint", "", commandAPITailscaleEndpointUsage)
	commandAPITailscale.AddCommand(commandAPITailscalePeer)
}

type tailscalePeerEntry struct {
	peer  *daemon.TailscalePeer
	group *daemon.TailscaleUserGroup
	self  bool
}

func tailscalePeerEntries(endpoint *daemon.TailscaleEndpointStatus) []tailscalePeerEntry {
	var entries []tailscalePeerEntry
	if endpoint.GetSelf() != nil {
		entries = append(entries, tailscalePeerEntry{peer: endpoint.GetSelf(), self: true})
	}
	for _, group := range endpoint.GetUserGroups() {
		for _, peer := range group.GetPeers() {
			entries = append(entries, tailscalePeerEntry{peer: peer, group: group})
		}
	}
	return entries
}

func resolveTailscalePeer(entries []tailscalePeerEntry, selector string) (tailscalePeerEntry, error) {
	selectorAddress, addressErr := netip.ParseAddr(selector)
	matchers := []func(peer *daemon.TailscalePeer) bool{
		func(peer *daemon.TailscalePeer) bool {
			return peer.GetStableID() == selector
		},
		func(peer *daemon.TailscalePeer) bool {
			if addressErr != nil {
				return false
			}
			return slices.ContainsFunc(peer.GetTailscaleIPs(), func(it string) bool {
				address, parseErr := netip.ParseAddr(it)
				if parseErr != nil {
					return false
				}
				return address.Unmap() == selectorAddress.Unmap()
			})
		},
		func(peer *daemon.TailscalePeer) bool {
			dnsName := peer.GetDnsName()
			if dnsName == "" {
				return false
			}
			return strings.EqualFold(dnsName, selector) || strings.EqualFold(dns.FqdnToDomain(dnsName), selector)
		},
		func(peer *daemon.TailscalePeer) bool {
			label, _, _ := strings.Cut(peer.GetDnsName(), ".")
			if label == "" {
				return false
			}
			return strings.EqualFold(label, selector)
		},
		func(peer *daemon.TailscalePeer) bool {
			hostName := peer.GetHostName()
			if hostName == "" {
				return false
			}
			return strings.EqualFold(hostName, selector)
		},
	}
	for _, matcher := range matchers {
		matches := common.Filter(entries, func(it tailscalePeerEntry) bool {
			return matcher(it.peer)
		})
		if len(matches) == 1 {
			return matches[0], nil
		}
		if len(matches) > 1 {
			return tailscalePeerEntry{}, newTailscaleAmbiguousPeerError(selector, matches)
		}
	}
	return tailscalePeerEntry{}, E.New("peer not found: ", selector)
}

func newTailscaleAmbiguousPeerError(selector string, matches []tailscalePeerEntry) error {
	sortTailscalePeerEntries(matches)
	names := common.Map(matches, func(it tailscalePeerEntry) string {
		return tailscalePeerName(it.peer)
	})
	addresses := common.Map(matches, func(it tailscalePeerEntry) string {
		address := tailscalePeerAddress(it.peer)
		if address == "" {
			return "-"
		}
		return address
	})
	nameWidth := len(common.MaxBy(names, func(it string) int {
		return len(it)
	}))
	addressWidth := len(common.MaxBy(addresses, func(it string) int {
		return len(it)
	}))
	var builder strings.Builder
	builder.WriteString("ambiguous peer: ")
	builder.WriteString(selector)
	for index, entry := range matches {
		builder.WriteString("\n  ")
		builder.WriteString(names[index])
		builder.WriteString(strings.Repeat(" ", nameWidth-len(names[index])+3))
		builder.WriteString(addresses[index])
		builder.WriteString(strings.Repeat(" ", addressWidth-len(addresses[index])+3))
		builder.WriteString(entry.peer.GetStableID())
	}
	return E.New(builder.String())
}

func sortTailscalePeerEntries(entries []tailscalePeerEntry) {
	common.SortBy(entries, func(it tailscalePeerEntry) string {
		return strings.ToLower(tailscalePeerName(it.peer))
	})
}

func tailscalePeerName(peer *daemon.TailscalePeer) string {
	name := dns.FqdnToDomain(peer.GetDnsName())
	if name == "" {
		name = peer.GetHostName()
	}
	if name == "" {
		name = peer.GetStableID()
	}
	return name
}

func tailscalePeerAddress(peer *daemon.TailscalePeer) string {
	addresses := peer.GetTailscaleIPs()
	if len(addresses) == 0 {
		return ""
	}
	return addresses[0]
}

func formatYesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
