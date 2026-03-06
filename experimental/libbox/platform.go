package libbox

import C "github.com/sagernet/sing-box/constant"

type PlatformInterface interface {
	LocalDNSTransport() LocalDNSTransport
	UsePlatformAutoDetectInterfaceControl() bool
	AutoDetectInterfaceControl(fd int32) error
	OpenTun(options TunOptions) (int32, error)
	UseProcFS() bool
	FindConnectionOwner(ipProtocol int32, sourceAddress string, sourcePort int32, destinationAddress string, destinationPort int32) (*ConnectionOwner, error)
	StartDefaultInterfaceMonitor(listener InterfaceUpdateListener) error
	CloseDefaultInterfaceMonitor(listener InterfaceUpdateListener) error
	GetInterfaces() (NetworkInterfaceIterator, error)
	UnderNetworkExtension() bool
	IncludeAllNetworks() bool
	ReadWIFIState() *WIFIState
	SystemCertificates() StringIterator
	ClearDNSCache()
	SendNotification(notification *Notification) error
	StartNeighborMonitor(listener NeighborUpdateListener) error
	CloseNeighborMonitor(listener NeighborUpdateListener) error
	RegisterMyInterface(name string)
}

type NeighborUpdateListener interface {
	UpdateNeighborTable(entries NeighborEntryIterator)
}

type ConnectionOwner struct {
	UserId              int32
	UserName            string
	ProcessPath         string
	androidPackageNames []string
}

func (c *ConnectionOwner) SetAndroidPackageNames(names StringIterator) {
	c.androidPackageNames = iteratorToArray[string](names)
}

func (c *ConnectionOwner) AndroidPackageNames() StringIterator {
	return newIterator(c.androidPackageNames)
}

type InterfaceUpdateListener interface {
	UpdateDefaultInterface(interfaceName string, interfaceIndex int32, isExpensive bool, isConstrained bool)
}

const (
	InterfaceTypeWIFI     = int32(C.InterfaceTypeWIFI)
	InterfaceTypeCellular = int32(C.InterfaceTypeCellular)
	InterfaceTypeEthernet = int32(C.InterfaceTypeEthernet)
	InterfaceTypeOther    = int32(C.InterfaceTypeOther)
)

type NetworkInterface struct {
	Index     int32
	MTU       int32
	Name      string
	Addresses StringIterator
	Flags     int32

	Type      int32
	DNSServer StringIterator
	Metered   bool
}

type WIFIState struct {
	SSID  string
	BSSID string
}

func NewWIFIState(wifiSSID string, wifiBSSID string) *WIFIState {
	return &WIFIState{wifiSSID, wifiBSSID}
}

type NetworkInterfaceIterator interface {
	Next() *NetworkInterface
	HasNext() bool
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

type OnDemandRule interface {
	Target() int32
	DNSSearchDomainMatch() StringIterator
	DNSServerAddressMatch() StringIterator
	InterfaceTypeMatch() int32
	SSIDMatch() StringIterator
	ProbeURL() string
}

type OnDemandRuleIterator interface {
	Next() OnDemandRule
	HasNext() bool
}
