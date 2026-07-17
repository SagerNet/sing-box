package adapter

import (
	"net/netip"
	"time"
)

const (
	OpenVPNStateConnecting  = "connecting"
	OpenVPNStateAuthPending = "auth-pending"
	OpenVPNStateConnected   = "connected"
	OpenVPNStateError       = "error"
)

type OpenVPNEndpoint interface {
	Endpoint
	OpenVPNStatus() OpenVPNStatus
	StatusUpdated() <-chan struct{}
	CompleteChallenge(challengeID string, response OpenVPNChallengeResponse) error
	CancelChallenge(challengeID string) error
}

type OpenVPNStatus struct {
	State      string
	Challenge  *OpenVPNChallenge
	Error      string
	TunnelInfo *OpenVPNTunnelInfo
}

type OpenVPNTunnelInfo struct {
	Server         string
	Network        string
	Cipher         string
	IPv4           []netip.Prefix
	IPv6           []netip.Prefix
	DNS            []netip.Addr
	MTU            uint32
	ConnectedSince time.Time
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
	Deadline      time.Time
}

type OpenVPNChallengeResponse struct {
	Username string
	Password string
	Secret   string
}
