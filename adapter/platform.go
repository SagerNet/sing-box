package adapter

import (
	"context"
	"net/netip"

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
	ProcessPlatformOptions(options option.TunPlatformOptions) error

	UsePlatformDefaultInterfaceMonitor() bool
	CreateDefaultInterfaceMonitor(logger logger.Logger) tun.DefaultInterfaceMonitor

	UsePlatformNetworkInterfaces() bool
	NetworkInterfaces() ([]NetworkInterface, error)

	UnderNetworkExtension() bool
	NetworkExtensionIncludeAllNetworks() bool

	ClearDNSCache()
	RequestPermissionForWIFIState() error
	ReadWIFIState(ctx context.Context) WIFIState

	UsePlatformConnectionOwnerFinder() bool
	FindConnectionOwner(request *FindConnectionOwnerRequest) (*ConnectionOwner, error)

	UsePlatformWIFIMonitor() bool

	UsePlatformNotification() bool
	SendNotification(notification *Notification) error
	CancelNotification(identifier string, typeID int32) error

	MyInterfaceAddress() []netip.Addr

	UsePlatformNeighborResolver() bool
	StartNeighborMonitor(listener NeighborUpdateListener) error
	CloseNeighborMonitor(listener NeighborUpdateListener) error

	UsePlatformShell() bool
	CheckPlatformShell() error
	OpenShellSession(user *PlatformUser, command string, env []string, term string, rows int32, cols int32) (ShellSession, error)
	LookupUser(username string) (*PlatformUser, error)
	LookupSFTPServer() (string, error)
	ReadSystemSSHHostKey() ([]byte, error)
	TailscaleHostname() string

	UsePlatformBridge() bool
	CreateBridge(options BridgeOptions) (BridgeSession, error)

	UsePlatformAutoRedirect() bool
	CreateAutoRedirect(options AutoRedirectOptions) (AutoRedirectSession, error)
}

type AutoRedirectOptions struct {
	TunOptions                     *tun.Options
	TableName                      string
	RedirectPort                   uint16
	RedirectListenerFileDescriptor func() (int, error)
	RouteAddressSetFileDescriptor  func() (int, error)
	Handler                        tun.AutoRedirectHandler
}

type AutoRedirectSession interface {
	Close() error
	UpdateRouteAddressSet() error
}

type BridgeOptions struct {
	BridgeName string
	MTU        uint32
	Inet4Port  netip.Addr
	Inet6Port  netip.Addr
	Interface  string
	RuleIndex  int
	RouteTable int
}

type BridgeSession interface {
	FileDescriptor() int
	Name() string
	Inet6Active() bool
	SetEgress(interfaceName string) error
	Close() error
}

type PlatformUser struct {
	Username string
	Uid      int
	Gid      int
	HomeDir  string
	Shell    string
	Groups   []int
}

type FindConnectionOwnerRequest struct {
	IpProtocol         int32
	SourceAddress      string
	SourcePort         int32
	DestinationAddress string
	DestinationPort    int32
}

type ConnectionOwner struct {
	ProcessID           uint32
	UserId              int32
	UserName            string
	ProcessPath         string
	AndroidPackageNames []string
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
