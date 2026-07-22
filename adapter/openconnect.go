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
	CompleteAuthChallenge(challengeID string, response OpenConnectAuthResponse) error
	CancelAuthChallenge(challengeID string) error
}

type OpenConnectStatus struct {
	State         string
	AuthChallenge *OpenConnectAuthChallenge
	Error         string
	TunnelInfo    *OpenConnectTunnelInfo
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

type OpenConnectAuthChallenge struct {
	ID      string
	Banner  string
	Message string
	Error   string
	Form    *OpenConnectAuthForm
	Browser *OpenConnectBrowserRequest
}

type OpenConnectAuthForm struct {
	Fields []OpenConnectAuthFormField
}

type OpenConnectBrowserRequest struct {
	URL                 string
	FinalURL            string
	CookieNames         []string
	EarlyCookieNames    []string
	HeaderNames         []string
	CallbackURLPrefixes []string
	CacheID             string
}

type OpenConnectBrowserCookie struct {
	Name  string
	Value string
}

type OpenConnectBrowserHeader struct {
	Name   string
	Values []string
}

type OpenConnectAuthResponse struct {
	Form    *OpenConnectAuthFormResponse
	Browser *OpenConnectBrowserResult
}

type OpenConnectAuthFormResponse struct {
	Values map[string]string
}

type OpenConnectBrowserResult struct {
	FinalURL string
	Cookies  []OpenConnectBrowserCookie
	Headers  []OpenConnectBrowserHeader
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
