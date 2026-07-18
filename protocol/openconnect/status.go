package openconnect

import (
	"net/http"
	"slices"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-openconnect"
	"github.com/sagernet/sing/common"
)

var _ adapter.OpenConnectEndpoint = (*Endpoint)(nil)

func (e *Endpoint) OpenConnectStatus() adapter.OpenConnectStatus {
	var status adapter.OpenConnectStatus
	clientState := e.state.Load()
	authChallenge := e.client.PendingAuthChallenge()
	e.statusAccess.Lock()
	status.Error = e.terminalError
	e.statusAccess.Unlock()
	if authChallenge != nil {
		challenge := &adapter.OpenConnectAuthChallenge{
			ID:      authChallenge.ID,
			Banner:  authChallenge.Banner,
			Message: authChallenge.Message,
			Error:   authChallenge.Error,
		}
		if authChallenge.Form != nil {
			challenge.Form = &adapter.OpenConnectAuthForm{
				Fields: common.Map(authChallenge.Form.Fields, func(field openconnect.AuthFormField) adapter.OpenConnectAuthFormField {
					return adapter.OpenConnectAuthFormField{
						SubmissionKey: field.SubmissionKey,
						Name:          field.Name,
						Label:         field.Label,
						Kind:          field.Kind,
						Value:         field.Value,
						Options: common.Map(field.Options, func(choice openconnect.AuthFormChoice) adapter.OpenConnectAuthFormChoice {
							return adapter.OpenConnectAuthFormChoice{
								Value: choice.Value,
								Label: choice.Label,
							}
						}),
					}
				}),
			}
		}
		if authChallenge.Browser != nil {
			challenge.Browser = &adapter.OpenConnectBrowserRequest{
				URL:         authChallenge.Browser.URL,
				FinalURL:    authChallenge.Browser.FinalURL,
				CookieNames: slices.Clone(authChallenge.Browser.CookieNames),
				HeaderNames: slices.Clone(authChallenge.Browser.HeaderNames),
			}
		}
		status.AuthChallenge = challenge
	}
	switch {
	case status.AuthChallenge != nil:
		status.State = adapter.OpenConnectStateAuthPending
	case status.Error != "":
		status.State = adapter.OpenConnectStateError
	case clientState.started && clientState.tunnelConfigured && e.client.Ready():
		status.State = adapter.OpenConnectStateConnected
		tunnelInfo := clientState.tunnelInfo
		tunnelInfo.IPv4 = slices.Clone(tunnelInfo.IPv4)
		tunnelInfo.IPv6 = slices.Clone(tunnelInfo.IPv6)
		tunnelInfo.DNS = slices.Clone(tunnelInfo.DNS)
		status.TunnelInfo = &tunnelInfo
	default:
		status.State = adapter.OpenConnectStateConnecting
	}
	return status
}

func (e *Endpoint) StatusUpdated() <-chan struct{} {
	e.statusAccess.Lock()
	defer e.statusAccess.Unlock()
	return e.statusUpdated
}

func (e *Endpoint) CompleteAuthChallenge(challengeID string, response adapter.OpenConnectAuthResponse) error {
	var authResponse openconnect.AuthResponse
	if response.Form != nil {
		authResponse.Form = &openconnect.AuthFormResponse{Values: response.Form.Values}
	}
	if response.Browser != nil {
		browserResult := &openconnect.BrowserResult{
			FinalURL: response.Browser.FinalURL,
			Cookies: common.Map(response.Browser.Cookies, func(cookie adapter.OpenConnectBrowserCookie) openconnect.BrowserCookie {
				return openconnect.BrowserCookie{Name: cookie.Name, Value: cookie.Value}
			}),
			Header: make(http.Header),
		}
		for _, header := range response.Browser.Headers {
			for _, value := range header.Values {
				browserResult.Header.Add(header.Name, value)
			}
		}
		authResponse.Browser = browserResult
	}
	return e.client.CompleteAuthChallenge(challengeID, authResponse)
}

func (e *Endpoint) CancelAuthChallenge(challengeID string) error {
	return e.client.CancelAuthChallenge(challengeID)
}

func (e *Endpoint) notifyStatusUpdated() {
	e.statusAccess.Lock()
	e.notifyStatusUpdatedLocked()
	e.statusAccess.Unlock()
}

func (e *Endpoint) notifyStatusUpdatedLocked() {
	close(e.statusUpdated)
	e.statusUpdated = make(chan struct{})
}

func (e *Endpoint) setTerminalError(err error) {
	e.statusAccess.Lock()
	e.terminalError = err.Error()
	e.notifyStatusUpdatedLocked()
	e.statusAccess.Unlock()
}

func (e *Endpoint) watchAuthForms() {
	defer close(e.authFormLoopDone)
	var loggedAuthChallengeID string
	for {
		authChallengeUpdated := e.client.AuthChallengeUpdated()
		authChallenge := e.client.PendingAuthChallenge()
		if authChallenge != nil && authChallenge.ID != loggedAuthChallengeID {
			loggedAuthChallengeID = authChallenge.ID
			if authChallenge.Browser != nil {
				e.logger.Info("waiting for browser authentication")
			} else {
				e.logger.Info("waiting for authentication")
			}
		}
		e.notifyStatusUpdated()
		select {
		case <-e.loopContext.Done():
			return
		case <-authChallengeUpdated:
		}
	}
}

func (e *Endpoint) watchActiveTransport() {
	defer close(e.activeTransportLoopDone)
	for {
		transportUpdated := e.client.ActiveTransportUpdated()
		transport := e.client.ActiveTransport()
		e.stateAccess.Lock()
		e.updateState(func(state *clientState) {
			state.tunnelInfo.Transport = transport
		})
		e.stateAccess.Unlock()
		e.notifyStatusUpdated()
		select {
		case <-e.loopContext.Done():
			return
		case <-transportUpdated:
		}
	}
}
