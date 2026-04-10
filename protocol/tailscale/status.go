//go:build with_gvisor

package tailscale

import (
	"context"
	"slices"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/tailscale/ipn"
	"github.com/sagernet/tailscale/ipn/ipnstate"
)

var _ adapter.TailscaleEndpoint = (*Endpoint)(nil)

func (t *Endpoint) SubscribeTailscaleStatus(ctx context.Context, fn func(*adapter.TailscaleEndpointStatus)) error {
	localBackend := t.server.ExportLocalBackend()
	sendStatus := func() {
		status := localBackend.Status()
		fn(convertTailscaleStatus(status))
	}
	sendStatus()
	localBackend.WatchNotifications(ctx, ipn.NotifyInitialState|ipn.NotifyInitialNetMap|ipn.NotifyRateLimit, nil, func(roNotify *ipn.Notify) (keepGoing bool) {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		if roNotify.State != nil || roNotify.NetMap != nil || roNotify.BrowseToURL != nil {
			sendStatus()
		}
		return true
	})
	return ctx.Err()
}

func convertTailscaleStatus(status *ipnstate.Status) *adapter.TailscaleEndpointStatus {
	result := &adapter.TailscaleEndpointStatus{
		BackendState: status.BackendState,
		AuthURL:      status.AuthURL,
	}
	if status.CurrentTailnet != nil {
		result.NetworkName = status.CurrentTailnet.Name
		result.MagicDNSSuffix = status.CurrentTailnet.MagicDNSSuffix
	}
	if status.Self != nil {
		result.Self = convertTailscalePeer(status.Self)
	}
	groupIndex := make(map[int64]*adapter.TailscaleUserGroup)
	for _, peerKey := range status.Peers() {
		peer := status.Peer[peerKey]
		userID := int64(peer.UserID)
		group, loaded := groupIndex[userID]
		if !loaded {
			group = &adapter.TailscaleUserGroup{
				UserID: userID,
			}
			if profile, hasProfile := status.User[peer.UserID]; hasProfile {
				group.LoginName = profile.LoginName
				group.DisplayName = profile.DisplayName
				group.ProfilePicURL = profile.ProfilePicURL
			}
			groupIndex[userID] = group
			result.UserGroups = append(result.UserGroups, group)
		}
		group.Peers = append(group.Peers, convertTailscalePeer(peer))
	}
	for _, group := range result.UserGroups {
		slices.SortStableFunc(group.Peers, func(a, b *adapter.TailscalePeer) int {
			if a.Online != b.Online {
				if a.Online {
					return -1
				}
				return 1
			}
			return 0
		})
	}
	return result
}

func convertTailscalePeer(peer *ipnstate.PeerStatus) *adapter.TailscalePeer {
	ips := make([]string, len(peer.TailscaleIPs))
	for i, ip := range peer.TailscaleIPs {
		ips[i] = ip.String()
	}
	var keyExpiry int64
	if peer.KeyExpiry != nil {
		keyExpiry = peer.KeyExpiry.Unix()
	}
	return &adapter.TailscalePeer{
		HostName:       peer.HostName,
		DNSName:        peer.DNSName,
		OS:             peer.OS,
		TailscaleIPs:   ips,
		Online:         peer.Online,
		ExitNode:       peer.ExitNode,
		ExitNodeOption: peer.ExitNodeOption,
		Active:         peer.Active,
		RxBytes:        peer.RxBytes,
		TxBytes:        peer.TxBytes,
		UserID:         int64(peer.UserID),
		KeyExpiry:      keyExpiry,
	}
}
