package libbox

import (
	"github.com/sagernet/sing-box/option"
)

type PlatformInterface interface {
	UsePlatformAutoDetectInterfaceControl() bool
	AutoDetectInterfaceControl(fd int32) error
	OpenTun(options TunOptions) (int32, error)
	WriteLog(message string)
	UseProcFS() bool
	FindConnectionOwner(ipProtocol int32, sourceAddress string, sourcePort int32, destinationAddress string, destinationPort int32) (int32, error)
	PackageNameByUid(uid int32) (string, error)
	UIDByPackageName(packageName string) (int32, error)
	UsePlatformDefaultInterfaceMonitor() bool
	StartDefaultInterfaceMonitor(listener InterfaceUpdateListener) error
	CloseDefaultInterfaceMonitor(listener InterfaceUpdateListener) error
	UsePlatformInterfaceGetter() bool
	GetInterfaces() (NetworkInterfaceIterator, error)
	UnderNetworkExtension() bool
	IncludeAllNetworks() bool
	ReadWIFIState() *WIFIState
	ClearDNSCache()
}

type TunInterface interface {
	FileDescriptor() int32
	Close() error
}

type InterfaceUpdateListener interface {
	UpdateDefaultInterface(interfaceName string, interfaceIndex int32)
}

type NetworkInterface struct {
	Index     int32
	MTU       int32
	Name      string
	Addresses StringIterator
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

type onDemandRule struct {
	option.OnDemandRule
}

func (r *onDemandRule) Target() int32 {
	if r.OnDemandRule.Action == nil {
		return -1
	}
	return int32(*r.OnDemandRule.Action)
}

func (r *onDemandRule) DNSSearchDomainMatch() StringIterator {
	return newIterator(r.OnDemandRule.DNSSearchDomainMatch)
}

func (r *onDemandRule) DNSServerAddressMatch() StringIterator {
	return newIterator(r.OnDemandRule.DNSServerAddressMatch)
}

func (r *onDemandRule) InterfaceTypeMatch() int32 {
	if r.OnDemandRule.InterfaceTypeMatch == nil {
		return -1
	}
	return int32(*r.OnDemandRule.InterfaceTypeMatch)
}

func (r *onDemandRule) SSIDMatch() StringIterator {
	return newIterator(r.OnDemandRule.SSIDMatch)
}

func (r *onDemandRule) ProbeURL() string {
	return r.OnDemandRule.ProbeURL
}
