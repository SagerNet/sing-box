package adapter

import "context"

type TailscaleEndpoint interface {
	SubscribeTailscaleStatus(ctx context.Context, fn func(*TailscaleEndpointStatus)) error
	StartTailscalePing(ctx context.Context, peerIP string, fn func(*TailscalePingResult)) error
}

type TailscalePingResult struct {
	LatencyMs      float64
	IsDirect       bool
	Endpoint       string
	DERPRegionID   int32
	DERPRegionCode string
	Error          string
}

type TailscaleEndpointStatus struct {
	BackendState   string
	AuthURL        string
	NetworkName    string
	MagicDNSSuffix string
	Self           *TailscalePeer
	UserGroups     []*TailscaleUserGroup
}

type TailscaleUserGroup struct {
	UserID        int64
	LoginName     string
	DisplayName   string
	ProfilePicURL string
	Peers         []*TailscalePeer
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
