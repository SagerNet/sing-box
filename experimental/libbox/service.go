package libbox

import (
	"context"
	"net/netip"
	"runtime"
	"sync"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/experimental/libbox/internal/procfs"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

var _ daemon.PlatformInterface = (*platformInterfaceWrapper)(nil)

type platformInterfaceWrapper struct {
	iif                    PlatformInterface
	useProcFS              bool
	networkManager         adapter.NetworkManager
	myTunName              string
	defaultInterfaceAccess sync.Mutex
	defaultInterface       *control.Interface
	isExpensive            bool
	isConstrained          bool
}

func (w *platformInterfaceWrapper) Initialize(networkManager adapter.NetworkManager) error {
	w.networkManager = networkManager
	return nil
}

func (w *platformInterfaceWrapper) UsePlatformAutoDetectInterfaceControl() bool {
	return w.iif.UsePlatformAutoDetectInterfaceControl()
}

func (w *platformInterfaceWrapper) AutoDetectInterfaceControl(fd int) error {
	return w.iif.AutoDetectInterfaceControl(int32(fd))
}

func (w *platformInterfaceWrapper) UsePlatformInterface() bool {
	return true
}

func (w *platformInterfaceWrapper) OpenInterface(options *tun.Options, platformOptions option.TunPlatformOptions) (tun.Tun, error) {
	if len(options.IncludeUID) > 0 || len(options.ExcludeUID) > 0 {
		return nil, E.New("platform: unsupported uid options")
	}
	if len(options.IncludeAndroidUser) > 0 {
		return nil, E.New("platform: unsupported android_user option")
	}
	routeRanges, err := options.BuildAutoRouteRanges(true)
	if err != nil {
		return nil, err
	}
	tunFd, err := w.iif.OpenTun(&tunOptions{options, routeRanges, platformOptions})
	if err != nil {
		return nil, err
	}
	options.Name, err = getTunnelName(tunFd)
	if err != nil {
		return nil, E.Cause(err, "query tun name")
	}
	options.InterfaceMonitor.RegisterMyInterface(options.Name)
	dupFd, err := dup(int(tunFd))
	if err != nil {
		return nil, E.Cause(err, "dup tun file descriptor")
	}
	options.FileDescriptor = dupFd
	w.myTunName = options.Name
	return tun.New(*options)
}

func (w *platformInterfaceWrapper) UsePlatformDefaultInterfaceMonitor() bool {
	return true
}

func (w *platformInterfaceWrapper) CreateDefaultInterfaceMonitor(logger logger.Logger) tun.DefaultInterfaceMonitor {
	return &platformDefaultInterfaceMonitor{
		platformInterfaceWrapper: w,
		logger:                   logger,
	}
}

func (w *platformInterfaceWrapper) UsePlatformNetworkInterfaces() bool {
	return true
}

func (w *platformInterfaceWrapper) NetworkInterfaces() ([]adapter.NetworkInterface, error) {
	interfaceIterator, err := w.iif.GetInterfaces()
	if err != nil {
		return nil, err
	}
	var interfaces []adapter.NetworkInterface
	for _, netInterface := range iteratorToArray[*NetworkInterface](interfaceIterator) {
		if netInterface.Name == w.myTunName {
			continue
		}
		w.defaultInterfaceAccess.Lock()
		// (GOOS=windows) SA4006: this value of `isDefault` is never used
		// Why not used?
		//nolint:staticcheck
		isDefault := w.defaultInterface != nil && int(netInterface.Index) == w.defaultInterface.Index
		w.defaultInterfaceAccess.Unlock()
		interfaces = append(interfaces, adapter.NetworkInterface{
			Interface: control.Interface{
				Index:     int(netInterface.Index),
				MTU:       int(netInterface.MTU),
				Name:      netInterface.Name,
				Addresses: common.Map(iteratorToArray[string](netInterface.Addresses), netip.MustParsePrefix),
				Flags:     linkFlags(uint32(netInterface.Flags)),
			},
			Type:        C.InterfaceType(netInterface.Type),
			DNSServers:  iteratorToArray[string](netInterface.DNSServer),
			Expensive:   netInterface.Metered || isDefault && w.isExpensive,
			Constrained: isDefault && w.isConstrained,
		})
	}
	return interfaces, nil
}

func (w *platformInterfaceWrapper) UnderNetworkExtension() bool {
	return w.iif.UnderNetworkExtension()
}

func (w *platformInterfaceWrapper) NetworkExtensionIncludeAllNetworks() bool {
	return w.iif.IncludeAllNetworks()
}

func (w *platformInterfaceWrapper) ClearDNSCache() {
	w.iif.ClearDNSCache()
}

func (w *platformInterfaceWrapper) RequestPermissionForWIFIState() error {
	return nil
}

func (w *platformInterfaceWrapper) ReadWIFIState() adapter.WIFIState {
	wifiState := w.iif.ReadWIFIState()
	if wifiState == nil {
		return adapter.WIFIState{}
	}
	return (adapter.WIFIState)(*wifiState)
}

func (w *platformInterfaceWrapper) SystemCertificates() []string {
	return iteratorToArray[string](w.iif.SystemCertificates())
}

func (w *platformInterfaceWrapper) UsePlatformConnectionOwnerFinder() bool {
	return true
}

func (w *platformInterfaceWrapper) FindConnectionOwner(request *adapter.FindConnectionOwnerRequest) (*adapter.ConnectionOwner, error) {
	var uid int32
	if w.useProcFS {
		var source netip.AddrPort
		var destination netip.AddrPort
		sourceAddr, _ := netip.ParseAddr(request.SourceAddress)
		source = netip.AddrPortFrom(sourceAddr, uint16(request.SourcePort))
		destAddr, _ := netip.ParseAddr(request.DestinationAddress)
		destination = netip.AddrPortFrom(destAddr, uint16(request.DestinationPort))

		var network string
		switch request.IpProtocol {
		case int32(syscall.IPPROTO_TCP):
			network = "tcp"
		case int32(syscall.IPPROTO_UDP):
			network = "udp"
		default:
			return nil, E.New("unknown protocol: ", request.IpProtocol)
		}

		uid = procfs.ResolveSocketByProcSearch(network, source, destination)
		if uid == -1 {
			return nil, E.New("procfs: not found")
		}
	} else {
		var err error
		uid, err = w.iif.FindConnectionOwner(request.IpProtocol, request.SourceAddress, request.SourcePort, request.DestinationAddress, request.DestinationPort)
		if err != nil {
			return nil, err
		}
	}
	packageName, _ := w.iif.PackageNameByUid(uid)
	return &adapter.ConnectionOwner{
		UserId:             uid,
		AndroidPackageName: packageName,
	}, nil
}

func (w *platformInterfaceWrapper) DisableColors() bool {
	return runtime.GOOS != "android"
}

func (w *platformInterfaceWrapper) UsePlatformNotification() bool {
	return true
}

func (w *platformInterfaceWrapper) SendNotification(notification *adapter.Notification) error {
	return w.iif.SendNotification((*Notification)(notification))
}

func (w *platformInterfaceWrapper) UsePlatformLocalDNSTransport() bool {
	return C.IsAndroid
}

func (w *platformInterfaceWrapper) LocalDNSTransport() dns.TransportConstructorFunc[option.LocalDNSServerOptions] {
	localTransport := w.iif.LocalDNSTransport()
	if localTransport == nil {
		return nil
	}
	return func(ctx context.Context, logger log.ContextLogger, tag string, options option.LocalDNSServerOptions) (adapter.DNSTransport, error) {
		return newPlatformTransport(localTransport, tag, options), nil
	}
}
