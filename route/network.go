package route

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"runtime"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/conntrack"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/winpowrprof"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

var _ adapter.NetworkManager = (*NetworkManager)(nil)

type NetworkManager struct {
	logger                 logger.ContextLogger
	interfaceFinder        *control.DefaultInterfaceFinder
	autoDetectInterface    bool
	defaultInterface       string
	defaultMark            uint32
	autoRedirectOutputMark uint32
	networkMonitor         tun.NetworkUpdateMonitor
	interfaceMonitor       tun.DefaultInterfaceMonitor
	packageManager         tun.PackageManager
	powerListener          winpowrprof.EventListener
	pauseManager           pause.Manager
	platformInterface      platform.Interface
	outboundManager        adapter.OutboundManager
	wifiState              adapter.WIFIState
	started                bool
}

func NewNetworkManager(ctx context.Context, logger logger.ContextLogger, routeOptions option.RouteOptions) (*NetworkManager, error) {
	nm := &NetworkManager{
		logger:              logger,
		interfaceFinder:     control.NewDefaultInterfaceFinder(),
		autoDetectInterface: routeOptions.AutoDetectInterface,
		defaultInterface:    routeOptions.DefaultInterface,
		defaultMark:         routeOptions.DefaultMark,
		pauseManager:        service.FromContext[pause.Manager](ctx),
		platformInterface:   service.FromContext[platform.Interface](ctx),
		outboundManager:     service.FromContext[adapter.OutboundManager](ctx),
	}
	usePlatformDefaultInterfaceMonitor := nm.platformInterface != nil && nm.platformInterface.UsePlatformDefaultInterfaceMonitor()
	enforceInterfaceMonitor := routeOptions.AutoDetectInterface
	if !usePlatformDefaultInterfaceMonitor {
		networkMonitor, err := tun.NewNetworkUpdateMonitor(logger)
		if !((err != nil && !enforceInterfaceMonitor) || errors.Is(err, os.ErrInvalid)) {
			if err != nil {
				return nil, E.Cause(err, "create network monitor")
			}
			nm.networkMonitor = networkMonitor
			networkMonitor.RegisterCallback(func() {
				_ = nm.interfaceFinder.Update()
			})
			interfaceMonitor, err := tun.NewDefaultInterfaceMonitor(nm.networkMonitor, logger, tun.DefaultInterfaceMonitorOptions{
				InterfaceFinder:       nm.interfaceFinder,
				OverrideAndroidVPN:    routeOptions.OverrideAndroidVPN,
				UnderNetworkExtension: nm.platformInterface != nil && nm.platformInterface.UnderNetworkExtension(),
			})
			if err != nil {
				return nil, E.New("auto_detect_interface unsupported on current platform")
			}
			interfaceMonitor.RegisterCallback(nm.notifyNetworkUpdate)
			nm.interfaceMonitor = interfaceMonitor
		}
	} else {
		interfaceMonitor := nm.platformInterface.CreateDefaultInterfaceMonitor(logger)
		interfaceMonitor.RegisterCallback(nm.notifyNetworkUpdate)
		nm.interfaceMonitor = interfaceMonitor
	}
	return nm, nil
}

func (r *NetworkManager) Start(stage adapter.StartStage) error {
	monitor := taskmonitor.New(r.logger, C.StartTimeout)
	switch stage {
	case adapter.StartStateInitialize:
		if r.interfaceMonitor != nil {
			monitor.Start("initialize interface monitor")
			err := r.interfaceMonitor.Start()
			monitor.Finish()
			if err != nil {
				return err
			}
		}
		if r.networkMonitor != nil {
			monitor.Start("initialize network monitor")
			err := r.networkMonitor.Start()
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
	return nil
}

func (r *NetworkManager) InterfaceFinder() control.InterfaceFinder {
	return r.interfaceFinder
}

func (r *NetworkManager) UpdateInterfaces() error {
	if r.platformInterface == nil || !r.platformInterface.UsePlatformInterfaceGetter() {
		return r.interfaceFinder.Update()
	} else {
		interfaces, err := r.platformInterface.Interfaces()
		if err != nil {
			return err
		}
		r.interfaceFinder.UpdateInterfaces(interfaces)
		return nil
	}
}

func (r *NetworkManager) DefaultInterface() string {
	return r.defaultInterface
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
			if C.IsLinux {
				interfaceName, interfaceIndex = r.interfaceMonitor.DefaultInterface(remoteAddr)
				if interfaceIndex == -1 {
					err = tun.ErrNoRoute
				}
			} else {
				interfaceIndex = r.interfaceMonitor.DefaultInterfaceIndex(remoteAddr)
				if interfaceIndex == -1 {
					err = tun.ErrNoRoute
				}
			}
			return
		})
	}
}

func (r *NetworkManager) DefaultMark() uint32 {
	return r.defaultMark
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

func (r *NetworkManager) ResetNetwork() {
	conntrack.Close()

	for _, outbound := range r.outboundManager.Outbounds() {
		listener, isListener := outbound.(adapter.InterfaceUpdateListener)
		if isListener {
			listener.InterfaceUpdated()
		}
	}
}

func (r *NetworkManager) notifyNetworkUpdate(event int) {
	if event == tun.EventNoRoute {
		r.pauseManager.NetworkPause()
		r.logger.Error("missing default interface")
	} else {
		r.pauseManager.NetworkWake()
		if C.IsAndroid && r.platformInterface == nil {
			var vpnStatus string
			if r.interfaceMonitor.AndroidVPNEnabled() {
				vpnStatus = "enabled"
			} else {
				vpnStatus = "disabled"
			}
			r.logger.Info("updated default interface ", r.interfaceMonitor.DefaultInterfaceName(netip.IPv4Unspecified()), ", index ", r.interfaceMonitor.DefaultInterfaceIndex(netip.IPv4Unspecified()), ", vpn ", vpnStatus)
		} else {
			r.logger.Info("updated default interface ", r.interfaceMonitor.DefaultInterfaceName(netip.IPv4Unspecified()), ", index ", r.interfaceMonitor.DefaultInterfaceIndex(netip.IPv4Unspecified()))
		}
		if r.platformInterface != nil {
			state := r.platformInterface.ReadWIFIState()
			if state != r.wifiState {
				r.wifiState = state
				if state.SSID == "" && state.BSSID == "" {
					r.logger.Info("updated WIFI state: disconnected")
				} else {
					r.logger.Info("updated WIFI state: SSID=", state.SSID, ", BSSID=", state.BSSID)
				}
			}
		}
	}

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
