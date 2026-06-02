package adapter

import "context"

type TailscaleEndpoint interface {
	SubscribeTailscaleStatus(ctx context.Context, fn func(*TailscaleEndpointStatus)) error
	StartTailscalePing(ctx context.Context, peerIP string, fn func(*TailscalePingResult)) error
	SetTailscaleExitNode(ctx context.Context, stableID string) error
	Logout(ctx context.Context) error
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
	ExitNode       *TailscalePeer
	UserGroups     []*TailscaleUserGroup
	KeyAuth        bool
}

type TailscaleUserGroup struct {
	UserID        int64
	LoginName     string
	DisplayName   string
	ProfilePicURL string
	Peers         []*TailscalePeer
}

type TailscalePeer struct {
	StableID       string
	HostName       string
	DNSName        string
	OS             string
	TailscaleIPs   []string
	SSHHostKeys    []string
	Online         bool
	ExitNode       bool
	ExitNodeOption bool
	ShareeNode     bool
	Expired        bool
	Active         bool
	RxBytes        int64
	TxBytes        int64
	UserID         int64
	KeyExpiry      int64
	LastSeen       int64
}

type ShellSession interface {
	MasterFD() int32
	Resize(rows int32, cols int32) error
	Signal(signal int32) error
	WaitExit() (int32, error)
	Close() error
}
