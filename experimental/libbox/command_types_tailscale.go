package libbox

import "github.com/sagernet/sing-box/daemon"

type TailscaleStatusUpdate struct {
	endpoints []*TailscaleEndpointStatus
}

func (u *TailscaleStatusUpdate) Endpoints() TailscaleEndpointStatusIterator {
	return newIterator(u.endpoints)
}

type TailscaleEndpointStatusIterator interface {
	Next() *TailscaleEndpointStatus
	HasNext() bool
}

type TailscaleEndpointStatus struct {
	EndpointTag    string
	BackendState   string
	AuthURL        string
	NetworkName    string
	MagicDNSSuffix string
	Self           *TailscalePeer
	userGroups     []*TailscaleUserGroup
}

func (s *TailscaleEndpointStatus) UserGroups() TailscaleUserGroupIterator {
	return newIterator(s.userGroups)
}

type TailscaleUserGroupIterator interface {
	Next() *TailscaleUserGroup
	HasNext() bool
}

type TailscaleUserGroup struct {
	UserID        int64
	LoginName     string
	DisplayName   string
	ProfilePicURL string
	peers         []*TailscalePeer
}

func (g *TailscaleUserGroup) Peers() TailscalePeerIterator {
	return newIterator(g.peers)
}

type TailscalePeerIterator interface {
	Next() *TailscalePeer
	HasNext() bool
}

type TailscalePeer struct {
	HostName       string
	DNSName        string
	OS             string
	tailscaleIPs   []string
	Online         bool
	ExitNode       bool
	ExitNodeOption bool
	Active         bool
	RxBytes        int64
	TxBytes        int64
	KeyExpiry      int64
}

func (p *TailscalePeer) TailscaleIPs() StringIterator {
	return newIterator(p.tailscaleIPs)
}

type TailscaleStatusHandler interface {
	OnStatusUpdate(status *TailscaleStatusUpdate)
	OnError(message string)
}

func tailscaleStatusUpdateFromGRPC(update *daemon.TailscaleStatusUpdate) *TailscaleStatusUpdate {
	endpoints := make([]*TailscaleEndpointStatus, len(update.Endpoints))
	for i, endpoint := range update.Endpoints {
		endpoints[i] = tailscaleEndpointStatusFromGRPC(endpoint)
	}
	return &TailscaleStatusUpdate{endpoints: endpoints}
}

func tailscaleEndpointStatusFromGRPC(status *daemon.TailscaleEndpointStatus) *TailscaleEndpointStatus {
	userGroups := make([]*TailscaleUserGroup, len(status.UserGroups))
	for i, group := range status.UserGroups {
		userGroups[i] = tailscaleUserGroupFromGRPC(group)
	}
	result := &TailscaleEndpointStatus{
		EndpointTag:    status.EndpointTag,
		BackendState:   status.BackendState,
		AuthURL:        status.AuthURL,
		NetworkName:    status.NetworkName,
		MagicDNSSuffix: status.MagicDNSSuffix,
		userGroups:     userGroups,
	}
	if status.Self != nil {
		result.Self = tailscalePeerFromGRPC(status.Self)
	}
	return result
}

func tailscaleUserGroupFromGRPC(group *daemon.TailscaleUserGroup) *TailscaleUserGroup {
	peers := make([]*TailscalePeer, len(group.Peers))
	for i, peer := range group.Peers {
		peers[i] = tailscalePeerFromGRPC(peer)
	}
	return &TailscaleUserGroup{
		UserID:        group.UserID,
		LoginName:     group.LoginName,
		DisplayName:   group.DisplayName,
		ProfilePicURL: group.ProfilePicURL,
		peers:         peers,
	}
}

func tailscalePeerFromGRPC(peer *daemon.TailscalePeer) *TailscalePeer {
	return &TailscalePeer{
		HostName:       peer.HostName,
		DNSName:        peer.DnsName,
		OS:             peer.Os,
		tailscaleIPs:   peer.TailscaleIPs,
		Online:         peer.Online,
		ExitNode:       peer.ExitNode,
		ExitNodeOption: peer.ExitNodeOption,
		Active:         peer.Active,
		RxBytes:        peer.RxBytes,
		TxBytes:        peer.TxBytes,
		KeyExpiry:      peer.KeyExpiry,
	}
}
