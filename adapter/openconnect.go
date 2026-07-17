package adapter

import (
	"net/netip"
	"time"
)

const (
	OpenConnectStateConnecting  = "connecting"
	OpenConnectStateAuthPending = "auth-pending"
	OpenConnectStateConnected   = "connected"
	OpenConnectStateError       = "error"
)

type OpenConnectEndpoint interface {
	Endpoint
	OpenConnectStatus() OpenConnectStatus
	StatusUpdated() <-chan struct{}
	CompleteAuthForm(formID string, values map[string]string) error
	CancelAuthForm(formID string) error
}

type OpenConnectStatus struct {
	State      string
	AuthForm   *OpenConnectAuthForm
	Error      string
	TunnelInfo *OpenConnectTunnelInfo
}

type OpenConnectTunnelInfo struct {
	Server         string
	Flavor         string
	Transport      string
	IPv4           []netip.Prefix
	IPv6           []netip.Prefix
	DNS            []netip.Addr
	MTU            uint32
	ConnectedSince time.Time
}

type OpenConnectAuthForm struct {
	ID      string
	Banner  string
	Message string
	Error   string
	URL     string
	Fields  []OpenConnectAuthFormField
}

type OpenConnectAuthFormField struct {
	SubmissionKey string
	Name          string
	Label         string
	Kind          string
	Value         string
	Options       []OpenConnectAuthFormChoice
}

type OpenConnectAuthFormChoice struct {
	Value string
	Label string
}
