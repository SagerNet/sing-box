package openconnect

import (
	"slices"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-openconnect"
	"github.com/sagernet/sing/common"
)

var _ adapter.OpenConnectEndpoint = (*Endpoint)(nil)

func (e *Endpoint) OpenConnectStatus() adapter.OpenConnectStatus {
	var status adapter.OpenConnectStatus
	clientState := e.state.Load()
	authForm := e.client.PendingAuthForm()
	e.statusAccess.Lock()
	status.Error = e.terminalError
	e.statusAccess.Unlock()
	if authForm != nil {
		fields := common.Map(authForm.Fields, func(field openconnect.AuthFormField) adapter.OpenConnectAuthFormField {
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
		})
		status.AuthForm = &adapter.OpenConnectAuthForm{
			ID:      authForm.ID,
			Banner:  authForm.Banner,
			Message: authForm.Message,
			Error:   authForm.Error,
			URL:     authForm.URL,
			Fields:  fields,
		}
	}
	switch {
	case status.AuthForm != nil:
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

func (e *Endpoint) CompleteAuthForm(formID string, values map[string]string) error {
	return e.client.CompleteAuthForm(formID, values)
}

func (e *Endpoint) CancelAuthForm(formID string) error {
	return e.client.CancelAuthForm(formID)
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
	var loggedAuthFormID string
	for {
		authFormUpdated := e.client.AuthFormUpdated()
		authForm := e.client.PendingAuthForm()
		if authForm != nil && authForm.ID != loggedAuthFormID {
			loggedAuthFormID = authForm.ID
			if authForm.URL != "" {
				e.logger.Info("waiting for authentication: ", authForm.URL)
			} else {
				e.logger.Info("waiting for authentication")
			}
		}
		e.notifyStatusUpdated()
		select {
		case <-e.loopContext.Done():
			return
		case <-authFormUpdated:
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
