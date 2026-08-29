package adapter

import (
	"context"
	"io"
	"time"
)

type TailscaleEndpoint interface {
	SubscribeTailscaleStatus(ctx context.Context, fn func(*TailscaleEndpointStatus)) error
	StartTailscalePing(ctx context.Context, peerIP string, fn func(*TailscalePingResult)) error
	SetTailscaleExitNode(ctx context.Context, stableID string) error
	Logout(ctx context.Context) error
	GetTailscaleCertificate(ctx context.Context, domain string, minValidity time.Duration) (certificatePEM []byte, privateKeyPEM []byte, err error)
	SubscribeTaildropInbox(ctx context.Context, fn func(*TaildropInbox)) error
	MarkTaildropInboxRead() error
	SendTaildropFile(ctx context.Context, peerStableID string, fileName string, size int64, content io.Reader, progress func(sentBytes int64)) error
	OpenTaildropFile(fileName string) (io.ReadCloser, int64, error)
	DeleteTaildropFile(fileName string) error
	CancelTaildropReceiving(senderID string, fileName string) error
}

type TaildropInbox struct {
	Files     []*TaildropFile
	Receiving []*TaildropReceivingFile
}

type TaildropFile struct {
	Name       string
	Size       int64
	SenderName string
	ModifiedAt int64
}

type TaildropReceivingFile struct {
	Name          string
	Size          int64
	ReceivedBytes int64
	SenderID      string
	SenderName    string
}

type TailscalePingResult struct {
	LatencyMs      float64
	IsDirect       bool
	Endpoint       string
	PeerRelay      string
	DERPRegionID   int32
	DERPRegionCode string
	Error          string
}

type TailscaleEndpointStatus struct {
	BackendState       string
	AuthURL            string
	NetworkName        string
	MagicDNSSuffix     string
	Self               *TailscalePeer
	ExitNode           *TailscalePeer
	UserGroups         []*TailscaleUserGroup
	KeyAuth            bool
	CanShareFiles      bool
	WaitingFileCount   int32
	ReceivingFileCount int32
	UnreadFileCount    int32
	CertDomains        []string
}

type TailscaleUserGroup struct {
	UserID        int64
	LoginName     string
	DisplayName   string
	ProfilePicURL string
	Peers         []*TailscalePeer
}

type TailscalePeer struct {
	StableID        string
	HostName        string
	DNSName         string
	OS              string
	TailscaleIPs    []string
	SSHHostKeys     []string
	Online          bool
	ExitNode        bool
	ExitNodeOption  bool
	ShareeNode      bool
	Expired         bool
	Active          bool
	CanReceiveFiles bool
	RxBytes         int64
	TxBytes         int64
	UserID          int64
	KeyExpiry       int64
	LastSeen        int64
}

type ShellSession interface {
	MasterFD() int32
	Resize(rows int32, cols int32) error
	Signal(signal int32) error
	WaitExit() (int32, error)
	Close() error
}
