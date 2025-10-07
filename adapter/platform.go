package adapter

import (
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/logger"
)

type PlatformInterface interface {
	Initialize(networkManager NetworkManager) error

	UsePlatformAutoDetectInterfaceControl() bool
	AutoDetectInterfaceControl(fd int) error

	UsePlatformInterface() bool
	OpenInterface(options *tun.Options, platformOptions option.TunPlatformOptions) (tun.Tun, error)

	UsePlatformDefaultInterfaceMonitor() bool
	CreateDefaultInterfaceMonitor(logger logger.Logger) tun.DefaultInterfaceMonitor

	UsePlatformNetworkInterfaces() bool
	NetworkInterfaces() ([]NetworkInterface, error)

	UnderNetworkExtension() bool
	NetworkExtensionIncludeAllNetworks() bool

	ClearDNSCache()
	RequestPermissionForWIFIState() error
	ReadWIFIState() WIFIState
	SystemCertificates() []string

	UsePlatformConnectionOwnerFinder() bool
	FindConnectionOwner(request *FindConnectionOwnerRequest) (*ConnectionOwner, error)

	UsePlatformNotification() bool
	SendNotification(notification *Notification) error
}

type FindConnectionOwnerRequest struct {
	IpProtocol         int32
	SourceAddress      string
	SourcePort         int32
	DestinationAddress string
	DestinationPort    int32
}

type ConnectionOwner struct {
	ProcessID          uint32
	UserId             int32
	UserName           string
	ProcessPath        string
	AndroidPackageName string
}

type Notification struct {
	Identifier string
	TypeName   string
	TypeID     int32
	Title      string
	Subtitle   string
	Body       string
	OpenURL    string
}

type SystemProxyStatus struct {
	Available bool
	Enabled   bool
}
