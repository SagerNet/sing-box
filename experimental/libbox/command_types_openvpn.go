package libbox

import (
	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing/common"
)

type OpenVPNStatusUpdate struct {
	endpoints []*OpenVPNEndpointStatus
}

func (u *OpenVPNStatusUpdate) Endpoints() OpenVPNEndpointStatusIterator {
	return newIterator(u.endpoints)
}

type OpenVPNEndpointStatusIterator interface {
	Next() *OpenVPNEndpointStatus
	HasNext() bool
}

type OpenVPNEndpointStatus struct {
	EndpointTag string
	State       string
	StateText   string
	Challenge   *OpenVPNChallenge
	Error       string
	TunnelInfo  *OpenVPNTunnelInfo
}

type OpenVPNTunnelInfo struct {
	Server         string
	Network        string
	Cipher         string
	ipv4           []string
	ipv6           []string
	dns            []string
	MTU            int32
	ConnectedSince int64
}

func (i *OpenVPNTunnelInfo) IPv4() StringIterator {
	return newIterator(i.ipv4)
}

func (i *OpenVPNTunnelInfo) IPv6() StringIterator {
	return newIterator(i.ipv6)
}

func (i *OpenVPNTunnelInfo) DNS() StringIterator {
	return newIterator(i.dns)
}

type OpenVPNChallenge struct {
	ID            string
	Kind          string
	Username      string
	Message       string
	URL           string
	SecretMessage string
	Echo          bool
	PreviousError string
	Deadline      int64
}

type OpenVPNChallengeResponse struct {
	Username string
	Password string
	Secret   string
}

type OpenVPNStatusHandler interface {
	OnStatusUpdate(status *OpenVPNStatusUpdate)
	OnError(message string)
}

type OpenVPNStatusSubscription struct {
	streamSession
}

func openVPNStatusUpdateFromGRPC(update *daemon.OpenVPNStatusUpdate) *OpenVPNStatusUpdate {
	return &OpenVPNStatusUpdate{
		endpoints: common.Map(update.Endpoints, openVPNEndpointStatusFromGRPC),
	}
}

func openVPNEndpointStatusFromGRPC(status *daemon.OpenVPNEndpointStatus) *OpenVPNEndpointStatus {
	result := &OpenVPNEndpointStatus{
		EndpointTag: status.EndpointTag,
		State:       status.State,
		StateText:   status.StateText,
		Error:       status.Error,
	}
	if status.Challenge != nil {
		result.Challenge = &OpenVPNChallenge{
			ID:            status.Challenge.Id,
			Kind:          status.Challenge.Kind,
			Username:      status.Challenge.Username,
			Message:       status.Challenge.Message,
			URL:           status.Challenge.Url,
			SecretMessage: status.Challenge.SecretMessage,
			Echo:          status.Challenge.Echo,
			PreviousError: status.Challenge.PreviousError,
			Deadline:      status.Challenge.Deadline,
		}
	}
	if status.TunnelInfo != nil {
		result.TunnelInfo = &OpenVPNTunnelInfo{
			Server:         status.TunnelInfo.Server,
			Network:        status.TunnelInfo.Network,
			Cipher:         status.TunnelInfo.Cipher,
			ipv4:           status.TunnelInfo.Ipv4,
			ipv6:           status.TunnelInfo.Ipv6,
			dns:            status.TunnelInfo.Dns,
			MTU:            int32(status.TunnelInfo.Mtu),
			ConnectedSince: status.TunnelInfo.ConnectedSince,
		}
	}
	return result
}
