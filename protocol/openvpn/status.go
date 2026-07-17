package openvpn

import (
	"slices"

	"github.com/sagernet/sing-box/adapter"
	ovpn "github.com/sagernet/sing-openvpn"
)

var _ adapter.OpenVPNEndpoint = (*ClientEndpoint)(nil)

func (c *ClientEndpoint) OpenVPNStatus() adapter.OpenVPNStatus {
	var status adapter.OpenVPNStatus
	challenge := c.client.PendingChallenge()
	state := c.state.Load()
	c.statusAccess.Lock()
	status.Error = c.terminalError
	c.statusAccess.Unlock()
	switch {
	case challenge != nil:
		status.State = adapter.OpenVPNStateAuthPending
		status.Challenge = &adapter.OpenVPNChallenge{
			ID:            challenge.ID,
			Kind:          string(challenge.Kind),
			Username:      challenge.Username,
			Message:       challenge.Message,
			URL:           challenge.URL,
			SecretMessage: challenge.SecretMessage,
			Echo:          challenge.Echo,
			PreviousError: challenge.PreviousError,
			Deadline:      challenge.Deadline,
		}
	case status.Error != "":
		status.State = adapter.OpenVPNStateError
	case state.started && state.tunnelConfigured && c.client.Ready():
		status.State = adapter.OpenVPNStateConnected
		tunnelInfo := state.tunnelInfo
		tunnelInfo.IPv4 = slices.Clone(tunnelInfo.IPv4)
		tunnelInfo.IPv6 = slices.Clone(tunnelInfo.IPv6)
		tunnelInfo.DNS = slices.Clone(tunnelInfo.DNS)
		status.TunnelInfo = &tunnelInfo
	default:
		status.State = adapter.OpenVPNStateConnecting
	}
	return status
}

func (c *ClientEndpoint) StatusUpdated() <-chan struct{} {
	c.statusAccess.Lock()
	defer c.statusAccess.Unlock()
	return c.statusUpdated
}

func (c *ClientEndpoint) CompleteChallenge(challengeID string, response adapter.OpenVPNChallengeResponse) error {
	return c.client.CompleteChallenge(challengeID, ovpn.ChallengeResponse{
		Username: response.Username,
		Password: response.Password,
		Secret:   response.Secret,
	})
}

func (c *ClientEndpoint) CancelChallenge(challengeID string) error {
	return c.client.CancelChallenge(challengeID)
}

func (c *ClientEndpoint) notifyStatusUpdated() {
	c.statusAccess.Lock()
	c.notifyStatusUpdatedLocked()
	c.statusAccess.Unlock()
}

func (c *ClientEndpoint) notifyStatusUpdatedLocked() {
	close(c.statusUpdated)
	c.statusUpdated = make(chan struct{})
}

func (c *ClientEndpoint) setTerminalError(err error) {
	c.statusAccess.Lock()
	c.terminalError = err.Error()
	c.notifyStatusUpdatedLocked()
	c.statusAccess.Unlock()
}

func (c *ClientEndpoint) watchChallenges() {
	defer close(c.challengeLoopDone)
	var loggedChallengeID string
	for {
		challengeUpdated := c.client.ChallengeUpdated()
		challenge := c.client.PendingChallenge()
		if challenge != nil && challenge.ID != loggedChallengeID {
			loggedChallengeID = challenge.ID
			c.logChallenge(challenge)
		}
		c.notifyStatusUpdated()
		select {
		case <-c.loopContext.Done():
			return
		case <-challengeUpdated:
		}
	}
}

func (c *ClientEndpoint) logChallenge(challenge *ovpn.Challenge) {
	switch challenge.Kind {
	case ovpn.ChallengeCredentials:
		c.logger.Info("waiting for credentials")
	case ovpn.ChallengeSecret:
		c.logger.Info("waiting for challenge response: ", challenge.Message)
	case ovpn.ChallengeMessage:
		c.logger.Info("authentication message: ", challenge.Message)
	case ovpn.ChallengeOpenURL:
		c.logger.Info("waiting for authentication: ", challenge.URL)
	}
}
