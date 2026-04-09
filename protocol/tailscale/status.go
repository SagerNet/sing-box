//go:build with_gvisor

package tailscale

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/tailscale/ipn"
	"github.com/sagernet/tailscale/ipn/ipnstate"
	"github.com/sagernet/tailscale/tailcfg"
)

var _ adapter.TailscaleStatusProvider = (*Endpoint)(nil)

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
	result.Users = make(map[int64]*adapter.TailscaleUser, len(status.User))
	for userID, profile := range status.User {
		result.Users[int64(userID)] = convertTailscaleUser(userID, profile)
	}
	result.Peers = make([]*adapter.TailscalePeer, 0, len(status.Peer))
	for _, peer := range status.Peer {
		result.Peers = append(result.Peers, convertTailscalePeer(peer))
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

func convertTailscaleUser(id tailcfg.UserID, profile tailcfg.UserProfile) *adapter.TailscaleUser {
	return &adapter.TailscaleUser{
		ID:            int64(id),
		LoginName:     profile.LoginName,
		DisplayName:   profile.DisplayName,
		ProfilePicURL: profile.ProfilePicURL,
	}
}
