package route

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/conntrack"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/winpowrprof"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"

	"golang.org/x/exp/slices"
)

var _ adapter.NetworkManager = (*NetworkManager)(nil)

type NetworkManager struct {
	logger            logger.ContextLogger
	interfaceFinder   *control.DefaultInterfaceFinder
	networkInterfaces common.TypedValue[[]adapter.NetworkInterface]

	autoDetectInterface    bool
	defaultOptions         adapter.NetworkOptions
	autoRedirectOutputMark uint32
	networkMonitor         tun.NetworkUpdateMonitor
	interfaceMonitor       tun.DefaultInterfaceMonitor
	packageManager         tun.PackageManager
	powerListener          winpowrprof.EventListener
	pauseManager           pause.Manager
	platformInterface      platform.Interface
	endpoint               adapter.EndpointManager
	inbound                adapter.InboundManager
	outbound               adapter.OutboundManager
	wifiState              adapter.WIFIState
	started                bool
}

func NewNetworkManager(ctx context.Context, logger logger.ContextLogger, routeOptions option.RouteOptions) (*NetworkManager, error) {
	defaultDomainResolver := common.PtrValueOrDefault(routeOptions.DefaultDomainResolver)
	if routeOptions.AutoDetectInterface && !(C.IsLinux || C.IsDarwin || C.IsWindows) {
		return nil, E.New("`auto_detect_interface` is only supported on Linux, Windows and macOS")
	} else if routeOptions.OverrideAndroidVPN && !C.IsAndroid {
		return nil, E.New("`override_android_vpn` is only supported on Android")
	} else if routeOptions.DefaultInterface != "" && !(C.IsLinux || C.IsDarwin || C.IsWindows) {
		return nil, E.New("`default_interface` is only supported on Linux, Windows and macOS")
	} else if routeOptions.DefaultMark != 0 && !C.IsLinux {
		return nil, E.New("`default_mark` is only supported on linux")
	}
	nm := &NetworkManager{
		logger:              logger,
		interfaceFinder:     control.NewDefaultInterfaceFinder(),
		autoDetectInterface: routeOptions.AutoDetectInterface,
		defaultOptions: adapter.NetworkOptions{
			BindInterface:  routeOptions.DefaultInterface,
			RoutingMark:    uint32(routeOptions.DefaultMark),
			DomainResolver: defaultDomainResolver.Server,
			DomainResolveOptions: adapter.DNSQueryOptions{
				Strategy:     C.DomainStrategy(defaultDomainResolver.Strategy),
				DisableCache: defaultDomainResolver.DisableCache,
				RewriteTTL:   defaultDomainResolver.RewriteTTL,
				ClientSubnet: defaultDomainResolver.ClientSubnet.Build(netip.Prefix{}),
			},
			NetworkStrategy:     (*C.NetworkStrategy)(routeOptions.DefaultNetworkStrategy),
			NetworkType:         common.Map(routeOptions.DefaultNetworkType, option.InterfaceType.Build),
			FallbackNetworkType: common.Map(routeOptions.DefaultFallbackNetworkType, option.InterfaceType.Build),
			FallbackDelay:       time.Duration(routeOptions.DefaultFallbackDelay),
		},
		pauseManager:      service.FromContext[pause.Manager](ctx),
		platformInterface: service.FromContext[platform.Interface](ctx),
		endpoint:          service.FromContext[adapter.EndpointManager](ctx),
		inbound:           service.FromContext[adapter.InboundManager](ctx),
		outbound:          service.FromContext[adapter.OutboundManager](ctx),
	}
	if routeOptions.DefaultNetworkStrategy != nil {
		if routeOptions.DefaultInterface != "" {
			return nil, E.New("`default_network_strategy` is conflict with `default_interface`")
		}
		if !routeOptions.AutoDetectInterface {
			return nil, E.New("`auto_detect_interface` is required by `default_network_strategy`")
		}
	}
	usePlatformDefaultInterfaceMonitor := nm.platformInterface != nil
	enforceInterfaceMonitor := routeOptions.AutoDetectInterface
	if !usePlatformDefaultInterfaceMonitor {
		networkMonitor, err := tun.NewNetworkUpdateMonitor(logger)
		if !((err != nil && !enforceInterfaceMonitor) || errors.Is(err, os.ErrInvalid)) {
			if err != nil {
				return nil, E.Cause(err, "create network monitor")
			}
			nm.networkMonitor = networkMonitor
			interfaceMonitor, err := tun.NewDefaultInterfaceMonitor(nm.networkMonitor, logger, tun.DefaultInterfaceMonitorOptions{
				InterfaceFinder:       nm.interfaceFinder,
				OverrideAndroidVPN:    routeOptions.OverrideAndroidVPN,
				UnderNetworkExtension: nm.platformInterface != nil && nm.platformInterface.UnderNetworkExtension(),
			})
			if err != nil {
				return nil, E.New("auto_detect_interface unsupported on current platform")
			}
			interfaceMonitor.RegisterCallback(nm.notifyInterfaceUpdate)
			nm.interfaceMonitor = interfaceMonitor
		}
	} else {
		interfaceMonitor := nm.platformInterface.CreateDefaultInterfaceMonitor(logger)
		interfaceMonitor.RegisterCallback(nm.notifyInterfaceUpdate)
		nm.interfaceMonitor = interfaceMonitor
	}
	return nm, nil
}

func (r *NetworkManager) Start(stage adapter.StartStage) error {
	monitor := taskmonitor.New(r.logger, C.StartTimeout)
	switch stage {
	case adapter.StartStateInitialize:
		if r.networkMonitor != nil {
			monitor.Start("initialize network monitor")
			err := r.networkMonitor.Start()
			monitor.Finish()
			if err != nil {
				return err
			}
		}
		if r.interfaceMonitor != nil {
			monitor.Start("initialize interface monitor")
			err := r.interfaceMonitor.Start()
			monitor.Finish()
			if err != nil {
				return err
			}
		}
	case adapter.StartStateStart:
		if runtime.GOOS == "windows" {
			powerListener, err := winpowrprof.NewEventListener(r.notifyWindowsPowerEvent)
			if err == nil {
				r.powerListener = powerListener
			} else {
				r.logger.Warn("initialize power listener: ", err)
			}
		}
		if r.powerListener != nil {
			monitor.Start("start power listener")
			err := r.powerListener.Start()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "start power listener")
			}
		}
		if C.IsAndroid && r.platformInterface == nil {
			monitor.Start("initialize package manager")
			packageManager, err := tun.NewPackageManager(tun.PackageManagerOptions{
				Callback: r,
				Logger:   r.logger,
			})
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "create package manager")
			}
			monitor.Start("start package manager")
			err = packageManager.Start()
			monitor.Finish()
			if err != nil {
				r.logger.Warn("initialize package manager: ", err)
			} else {
				r.packageManager = packageManager
			}
		}
	case adapter.StartStatePostStart:
		r.started = true
	}
	return nil
}

func (r *NetworkManager) Close() error {
	monitor := taskmonitor.New(r.logger, C.StopTimeout)
	var err error
	if r.packageManager != nil {
		monitor.Start("close package manager")
		err = E.Append(err, r.packageManager.Close(), func(err error) error {
			return E.Cause(err, "close package manager")
		})
		monitor.Finish()
	}
	if r.powerListener != nil {
		monitor.Start("close power listener")
		err = E.Append(err, r.powerListener.Close(), func(err error) error {
			return E.Cause(err, "close power listener")
		})
		monitor.Finish()
	}
	if r.interfaceMonitor != nil {
		monitor.Start("close interface monitor")
		err = E.Append(err, r.interfaceMonitor.Close(), func(err error) error {
			return E.Cause(err, "close interface monitor")
		})
		monitor.Finish()
	}
	if r.networkMonitor != nil {
		monitor.Start("close network monitor")
		err = E.Append(err, r.networkMonitor.Close(), func(err error) error {
			return E.Cause(err, "close network monitor")
		})
		monitor.Finish()
	}
	return err
}

func (r *NetworkManager) InterfaceFinder() control.InterfaceFinder {
	return r.interfaceFinder
}

func (r *NetworkManager) UpdateInterfaces() error {
	if r.platformInterface == nil {
		return r.interfaceFinder.Update()
	} else {
		interfaces, err := r.platformInterface.Interfaces()
		if err != nil {
			return err
		}
		if C.IsDarwin {
			err = r.interfaceFinder.Update()
			if err != nil {
				return err
			}
			// NEInterface only provides name,index and type
			interfaces = common.Map(interfaces, func(it adapter.NetworkInterface) adapter.NetworkInterface {
				iif, _ := r.interfaceFinder.ByIndex(it.Index)
				if iif != nil {
					it.Interface = *iif
				}
				return it
			})
		} else {
			r.interfaceFinder.UpdateInterfaces(common.Map(interfaces, func(it adapter.NetworkInterface) control.Interface { return it.Interface }))
		}
		oldInterfaces := r.networkInterfaces.Load()
		newInterfaces := common.Filter(interfaces, func(it adapter.NetworkInterface) bool {
			return it.Flags&net.FlagUp != 0
		})
		r.networkInterfaces.Store(newInterfaces)
		if len(newInterfaces) > 0 && !slices.EqualFunc(oldInterfaces, newInterfaces, func(oldInterface adapter.NetworkInterface, newInterface adapter.NetworkInterface) bool {
			return oldInterface.Interface.Index == newInterface.Interface.Index &&
				oldInterface.Interface.Name == newInterface.Interface.Name &&
				oldInterface.Interface.Flags == newInterface.Interface.Flags &&
				oldInterface.Type == newInterface.Type &&
				oldInterface.Expensive == newInterface.Expensive &&
				oldInterface.Constrained == newInterface.Constrained
		}) {
			r.logger.Info("updated available networks: ", strings.Join(common.Map(newInterfaces, func(it adapter.NetworkInterface) string {
				var options []string
				options = append(options, F.ToString(it.Type))
				if it.Expensive {
					options = append(options, "expensive")
				}
				if it.Constrained {
					options = append(options, "constrained")
				}
				return F.ToString(it.Name, " (", strings.Join(options, ", "), ")")
			}), ", "))
		}
		return nil
	}
}

func (r *NetworkManager) DefaultNetworkInterface() *adapter.NetworkInterface {
	iif := r.interfaceMonitor.DefaultInterface()
	if iif == nil {
		return nil
	}
	for _, it := range r.networkInterfaces.Load() {
		if it.Interface.Index == iif.Index {
			return &it
		}
	}
	return &adapter.NetworkInterface{Interface: *iif}
}

func (r *NetworkManager) NetworkInterfaces() []adapter.NetworkInterface {
	return r.networkInterfaces.Load()
}

func (r *NetworkManager) AutoDetectInterface() bool {
	return r.autoDetectInterface
}

func (r *NetworkManager) AutoDetectInterfaceFunc() control.Func {
	if r.platformInterface != nil && r.platformInterface.UsePlatformAutoDetectInterfaceControl() {
		return func(network, address string, conn syscall.RawConn) error {
			return control.Raw(conn, func(fd uintptr) error {
				return r.platformInterface.AutoDetectInterfaceControl(int(fd))
			})
		}
	} else {
		if r.interfaceMonitor == nil {
			return nil
		}
		return control.BindToInterfaceFunc(r.interfaceFinder, func(network string, address string) (interfaceName string, interfaceIndex int, err error) {
			remoteAddr := M.ParseSocksaddr(address).Addr
			if remoteAddr.IsValid() {
				iif, err := r.interfaceFinder.ByAddr(remoteAddr)
				if err == nil {
					return iif.Name, iif.Index, nil
				}
			}
			defaultInterface := r.interfaceMonitor.DefaultInterface()
			if defaultInterface == nil {
				return "", -1, tun.ErrNoRoute
			}
			return defaultInterface.Name, defaultInterface.Index, nil
		})
	}
}

func (r *NetworkManager) ProtectFunc() control.Func {
	if r.platformInterface != nil && r.platformInterface.UsePlatformAutoDetectInterfaceControl() {
		return func(network, address string, conn syscall.RawConn) error {
			return control.Raw(conn, func(fd uintptr) error {
				return r.platformInterface.AutoDetectInterfaceControl(int(fd))
			})
		}
	}
	return nil
}

func (r *NetworkManager) DefaultOptions() adapter.NetworkOptions {
	return r.defaultOptions
}

func (r *NetworkManager) RegisterAutoRedirectOutputMark(mark uint32) error {
	if r.autoRedirectOutputMark > 0 {
		return E.New("only one auto-redirect can be configured")
	}
	r.autoRedirectOutputMark = mark
	return nil
}

func (r *NetworkManager) AutoRedirectOutputMark() uint32 {
	return r.autoRedirectOutputMark
}

func (r *NetworkManager) AutoRedirectOutputMarkFunc() control.Func {
	return func(network, address string, conn syscall.RawConn) error {
		if r.autoRedirectOutputMark == 0 {
			return nil
		}
		return control.RoutingMark(r.autoRedirectOutputMark)(network, address, conn)
	}
}

func (r *NetworkManager) NetworkMonitor() tun.NetworkUpdateMonitor {
	return r.networkMonitor
}

func (r *NetworkManager) InterfaceMonitor() tun.DefaultInterfaceMonitor {
	return r.interfaceMonitor
}

func (r *NetworkManager) PackageManager() tun.PackageManager {
	return r.packageManager
}

func (r *NetworkManager) WIFIState() adapter.WIFIState {
	return r.wifiState
}

func (r *NetworkManager) UpdateWIFIState() {
	if r.platformInterface != nil {
		state := r.platformInterface.ReadWIFIState()
		if state != r.wifiState {
			r.wifiState = state
			if state.SSID != "" {
				r.logger.Info("updated WIFI state: SSID=", state.SSID, ", BSSID=", state.BSSID)
			}
		}
	}
}

func (r *NetworkManager) ResetNetwork() {
	conntrack.Close()

	for _, endpoint := range r.endpoint.Endpoints() {
		listener, isListener := endpoint.(adapter.InterfaceUpdateListener)
		if isListener {
			listener.InterfaceUpdated()
		}
	}

	for _, inbound := range r.inbound.Inbounds() {
		listener, isListener := inbound.(adapter.InterfaceUpdateListener)
		if isListener {
			listener.InterfaceUpdated()
		}
	}

	for _, outbound := range r.outbound.Outbounds() {
		listener, isListener := outbound.(adapter.InterfaceUpdateListener)
		if isListener {
			listener.InterfaceUpdated()
		}
	}
}

func (r *NetworkManager) notifyInterfaceUpdate(defaultInterface *control.Interface, flags int) {
	if defaultInterface == nil {
		r.pauseManager.NetworkPause()
		r.logger.Error("missing default interface")
		return
	}

	r.pauseManager.NetworkWake()
	var options []string
	options = append(options, F.ToString("index ", defaultInterface.Index))
	if C.IsAndroid && r.platformInterface == nil {
		var vpnStatus string
		if r.interfaceMonitor.AndroidVPNEnabled() {
			vpnStatus = "enabled"
		} else {
			vpnStatus = "disabled"
		}
		options = append(options, "vpn "+vpnStatus)
	} else if r.platformInterface != nil {
		networkInterface := common.Find(r.networkInterfaces.Load(), func(it adapter.NetworkInterface) bool {
			return it.Interface.Index == defaultInterface.Index
		})
		if networkInterface.Name == "" {
			// race
			return
		}
		options = append(options, F.ToString("type ", networkInterface.Type))
		if networkInterface.Expensive {
			options = append(options, "expensive")
		}
		if networkInterface.Constrained {
			options = append(options, "constrained")
		}
	}
	r.logger.Info("updated default interface ", defaultInterface.Name, ", ", strings.Join(options, ", "))
	r.UpdateWIFIState()

	if !r.started {
		return
	}
	r.ResetNetwork()
}

func (r *NetworkManager) notifyWindowsPowerEvent(event int) {
	switch event {
	case winpowrprof.EVENT_SUSPEND:
		r.pauseManager.DevicePause()
		r.ResetNetwork()
	case winpowrprof.EVENT_RESUME:
		if !r.pauseManager.IsDevicePaused() {
			return
		}
		fallthrough
	case winpowrprof.EVENT_RESUME_AUTOMATIC:
		r.pauseManager.DeviceWake()
		r.ResetNetwork()
	}
}

func (r *NetworkManager) OnPackagesUpdated(packages int, sharedUsers int) {
	r.logger.Info("updated packages list: ", packages, " packages, ", sharedUsers, " shared users")
}
