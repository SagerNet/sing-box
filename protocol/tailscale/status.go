//go:build with_gvisor

package tailscale

import (
	"context"
	"slices"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/tailscale/ipn"
	"github.com/sagernet/tailscale/ipn/ipnstate"
)

var _ adapter.TailscaleEndpoint = (*Endpoint)(nil)

func (t *Endpoint) SubscribeTailscaleStatus(ctx context.Context, fn func(*adapter.TailscaleEndpointStatus)) error {
	localBackend := t.server.ExportLocalBackend()
	// The notification callback must stay cheap and non-blocking: a
	// watcher whose queue fills is disconnected by the IPN bus, so
	// status collection and delivery (which blocks on the subscriber)
	// run on a separate coalescing goroutine.
	updateSignal := make(chan struct{}, 1)
	scheduleUpdate := func() {
		select {
		case updateSignal <- struct{}{}:
		default:
		}
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-updateSignal:
			}
			status := localBackend.Status()
			result := convertTailscaleStatus(status)
			result.KeyAuth = t.keyAuth
			fn(result)
		}
	}()
	scheduleUpdate()
	for {
		var busError string
		localBackend.WatchNotifications(ctx, ipn.NotifyInitialState|ipn.NotifyPeerPatches, nil, func(roNotify *ipn.Notify) (keepGoing bool) {
			if roNotify.ErrMessage != nil {
				busError = *roNotify.ErrMessage
				return false
			}
			if roNotify.State != nil || roNotify.SelfChange != nil ||
				len(roNotify.PeersChanged) > 0 || len(roNotify.PeersRemoved) > 0 || len(roNotify.PeerChangedPatch) > 0 ||
				roNotify.BrowseToURL != nil || roNotify.Prefs != nil {
				scheduleUpdate()
			}
			return true
		})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if busError != "" {
			t.logger.Warn("restarting status watcher: ", busError)
		} else {
			t.logger.Warn("status watcher stopped unexpectedly, restarting")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
		scheduleUpdate()
	}
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
	// status.ExitNodeStatus is populated from the cached netmap Peers
	// slice, which incremental deltas do not update; the live peer map
	// behind status.Peer sets PeerStatus.ExitNode delta-correctly, so it
	// is the primary source.
	for _, peerKey := range status.Peers() {
		peer := status.Peer[peerKey]
		if peer.ExitNode {
			result.ExitNode = convertTailscalePeer(peer)
			break
		}
	}
	if result.ExitNode == nil && status.ExitNodeStatus != nil {
		ips := make([]string, 0, len(status.ExitNodeStatus.TailscaleIPs))
		for _, prefix := range status.ExitNodeStatus.TailscaleIPs {
			ips = append(ips, prefix.Addr().String())
		}
		result.ExitNode = &adapter.TailscalePeer{
			StableID:     string(status.ExitNodeStatus.ID),
			TailscaleIPs: ips,
			Online:       status.ExitNodeStatus.Online,
			ExitNode:     true,
		}
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
	var lastSeen int64
	if !peer.LastSeen.IsZero() {
		lastSeen = peer.LastSeen.Unix()
	}
	return &adapter.TailscalePeer{
		StableID:       string(peer.ID),
		HostName:       peer.HostName,
		DNSName:        peer.DNSName,
		OS:             peer.OS,
		TailscaleIPs:   ips,
		SSHHostKeys:    peer.SSH_HostKeys,
		Online:         peer.Online,
		ExitNode:       peer.ExitNode,
		ExitNodeOption: peer.ExitNodeOption,
		ShareeNode:     peer.ShareeNode,
		Expired:        peer.Expired,
		Active:         peer.Active,
		RxBytes:        peer.RxBytes,
		TxBytes:        peer.TxBytes,
		UserID:         int64(peer.UserID),
		KeyExpiry:      keyExpiry,
		LastSeen:       lastSeen,
	}
}
