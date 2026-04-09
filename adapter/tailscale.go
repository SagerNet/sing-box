package adapter

import "context"

type TailscaleStatusProvider interface {
	SubscribeTailscaleStatus(ctx context.Context, fn func(*TailscaleEndpointStatus)) error
}

type TailscaleEndpointStatus struct {
	BackendState   string
	AuthURL        string
	NetworkName    string
	MagicDNSSuffix string
	Self           *TailscalePeer
	Users          map[int64]*TailscaleUser
	Peers          []*TailscalePeer
}

type TailscalePeer struct {
	HostName       string
	DNSName        string
	OS             string
	TailscaleIPs   []string
	Online         bool
	ExitNode       bool
	ExitNodeOption bool
	Active         bool
	RxBytes        int64
	TxBytes        int64
	UserID         int64
	KeyExpiry      int64
}

type TailscaleUser struct {
	ID            int64
	LoginName     string
	DisplayName   string
	ProfilePicURL string
}
