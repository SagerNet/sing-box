package libbox

import (
	"context"
	"net/netip"
	"runtime"
	"syscall"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/common/process"
	"github.com/sagernet/sing-box/experimental/libbox/internal/procfs"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

type BoxService struct {
	ctx      context.Context
	cancel   context.CancelFunc
	instance *box.Box
}

func NewService(configContent string, platformInterface PlatformInterface) (*BoxService, error) {
	options, err := parseConfig(configContent)
	if err != nil {
		return nil, err
	}
	platformInterface.WriteLog("Hello " + runtime.GOOS + "/" + runtime.GOARCH)
	options.PlatformInterface = &platformInterfaceWrapper{platformInterface, platformInterface.UseProcFS()}
	ctx, cancel := context.WithCancel(context.Background())
	instance, err := box.New(ctx, options)
	if err != nil {
		cancel()
		return nil, E.Cause(err, "create service")
	}
	return &BoxService{
		ctx:      ctx,
		cancel:   cancel,
		instance: instance,
	}, nil
}

func (s *BoxService) Start() error {
	return s.instance.Start()
}

func (s *BoxService) Close() error {
	s.cancel()
	return s.instance.Close()
}

var _ platform.Interface = (*platformInterfaceWrapper)(nil)

type platformInterfaceWrapper struct {
	iif       PlatformInterface
	useProcFS bool
}

func (w *platformInterfaceWrapper) AutoDetectInterfaceControl() control.Func {
	return func(network, address string, conn syscall.RawConn) error {
		return control.Raw(conn, func(fd uintptr) error {
			return w.iif.AutoDetectInterfaceControl(int32(fd))
		})
	}
}

func (w *platformInterfaceWrapper) OpenTun(options tun.Options) (tun.Tun, error) {
	if len(options.IncludeUID) > 0 || len(options.ExcludeUID) > 0 {
		return nil, E.New("android: unsupported uid options")
	}
	if len(options.IncludeAndroidUser) > 0 {
		return nil, E.New("android: unsupported android_user option")
	}

	optionsWrapper := tunOptions(options)
	tunFd, err := w.iif.OpenTun(&optionsWrapper)
	if err != nil {
		return nil, err
	}
	options.FileDescriptor = int(tunFd)
	return tun.New(options)
}

func (w *platformInterfaceWrapper) Write(p []byte) (n int, err error) {
	w.iif.WriteLog(string(p))
	return len(p), nil
}

func (w *platformInterfaceWrapper) FindProcessInfo(ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*process.Info, error) {
	var uid int32
	if w.useProcFS {
		uid = procfs.ResolveSocketByProcSearch(network, source, destination)
		if uid == -1 {
			return nil, E.New("procfs: not found")
		}
	} else {
		var ipProtocol int32
		switch N.NetworkName(network) {
		case N.NetworkTCP:
			ipProtocol = syscall.IPPROTO_TCP
		case N.NetworkUDP:
			ipProtocol = syscall.IPPROTO_UDP
		default:
			return nil, E.New("unknown network: ", network)
		}
		var err error
		uid, err = w.iif.FindConnectionOwner(ipProtocol, source.Addr().String(), int32(source.Port()), destination.Addr().String(), int32(destination.Port()))
		if err != nil {
			return nil, err
		}
	}
	packageName, _ := w.iif.PackageNameByUid(uid)
	return &process.Info{UserId: uid, PackageName: packageName}, nil
}
