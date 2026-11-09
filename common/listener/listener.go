package listener

import (
	"context"
	"net"
	"net/netip"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/settings"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"

	"github.com/vishvananda/netns"
)

type Listener struct {
	ctx                      context.Context
	logger                   logger.ContextLogger
	network                  []string
	listenOptions            option.ListenOptions
	connHandler              adapter.ConnectionHandler
	packetHandler            adapter.PacketHandler
	oobPacketHandler         adapter.OOBPacketHandler
	threadUnsafePacketWriter bool
	disablePacketOutput      bool
	setSystemProxy           bool
	systemProxySOCKS         bool
	tproxy                   bool

	tcpListener          net.Listener
	systemProxy          settings.SystemProxy
	udpConn              *net.UDPConn
	udpAddr              M.Socksaddr
	packetOutbound       chan *N.PacketBuffer
	packetOutboundClosed chan struct{}
	shutdown             atomic.Bool
	interfaceCallbacks   []*list.Element[control.InterfaceUpdateCallback]
}

type Options struct {
	Context                  context.Context
	Logger                   logger.ContextLogger
	Network                  []string
	Listen                   option.ListenOptions
	ConnectionHandler        adapter.ConnectionHandler
	PacketHandler            adapter.PacketHandler
	OOBPacketHandler         adapter.OOBPacketHandler
	ThreadUnsafePacketWriter bool
	DisablePacketOutput      bool
	SetSystemProxy           bool
	SystemProxySOCKS         bool
	TProxy                   bool
}

func New(
	options Options,
) *Listener {
	return &Listener{
		ctx:                      options.Context,
		logger:                   options.Logger,
		network:                  options.Network,
		listenOptions:            options.Listen,
		connHandler:              options.ConnectionHandler,
		packetHandler:            options.PacketHandler,
		oobPacketHandler:         options.OOBPacketHandler,
		threadUnsafePacketWriter: options.ThreadUnsafePacketWriter,
		disablePacketOutput:      options.DisablePacketOutput,
		setSystemProxy:           options.SetSystemProxy,
		systemProxySOCKS:         options.SystemProxySOCKS,
		tproxy:                   options.TProxy,
	}
}

func (l *Listener) Start() error {
	if common.Contains(l.network, N.NetworkTCP) {
		_, err := l.ListenTCP()
		if err != nil {
			return err
		}
		go l.loopTCPIn()
	}
	if common.Contains(l.network, N.NetworkUDP) {
		_, err := l.ListenUDP()
		if err != nil {
			return E.Errors(err, l.Close())
		}
		l.packetOutboundClosed = make(chan struct{})
		l.packetOutbound = make(chan *N.PacketBuffer, 64)
		go l.loopUDPIn()
		if !l.disablePacketOutput {
			go l.loopUDPOut()
		}
	}
	if l.setSystemProxy {
		listenPort := M.SocksaddrFromNet(l.tcpListener.Addr()).Port
		var listenAddrString string
		listenAddr := l.listenOptions.Listen.Build(netip.IPv4Unspecified())
		if listenAddr.IsUnspecified() {
			listenAddrString = "127.0.0.1"
		} else {
			listenAddrString = listenAddr.String()
		}
		systemProxy, err := settings.NewSystemProxy(l.ctx, M.ParseSocksaddrHostPort(listenAddrString, listenPort), l.systemProxySOCKS, nil)
		if err != nil {
			return E.Errors(E.Cause(err, "initialize system proxy"), l.Close())
		}
		err = systemProxy.Enable()
		if err != nil {
			return E.Errors(E.Cause(err, "set system proxy"), systemProxy.Close(), l.Close())
		}
		l.systemProxy = systemProxy
	}
	return nil
}

func (l *Listener) Close() error {
	l.shutdown.Store(true)
	if len(l.interfaceCallbacks) > 0 {
		interfaceFinder := service.FromContext[adapter.NetworkManager](l.ctx).InterfaceFinder()
		for _, element := range l.interfaceCallbacks {
			interfaceFinder.UnregisterInterfaceUpdateCallback(element)
		}
	}
	var err error
	if l.systemProxy != nil {
		if l.systemProxy.IsEnabled() {
			err = l.systemProxy.Disable()
		}
		err = E.Errors(err, l.systemProxy.Close())
	}
	return E.Errors(err, common.Close(
		l.tcpListener,
		common.PtrOrNil(l.udpConn),
	))
}

func (l *Listener) bindInterfaceFunc() control.Func {
	if l.listenOptions.NetNs != "" {
		return func(network, address string, conn syscall.RawConn) error {
			return control.BindToInterface0(nil, conn, network, address, l.listenOptions.BindInterface, -1, true)
		}
	}
	return control.BindToInterface(service.FromContext[adapter.NetworkManager](l.ctx).InterfaceFinder(), l.listenOptions.BindInterface, -1)
}

func (l *Listener) followBindInterface() (control.Func, func(conn syscall.Conn)) {
	if l.listenOptions.NetNs != "" {
		return l.bindInterfaceFunc(), nil
	}
	interfaceName := l.listenOptions.BindInterface
	interfaceFinder := service.FromContext[adapter.NetworkManager](l.ctx).InterfaceFinder()
	var (
		access         sync.Mutex
		bindNetwork    string
		bindAddress    string
		interfaceIndex int
	)
	bindFunc := func(network, address string, conn syscall.RawConn) error {
		iif, err := interfaceFinder.ByName(interfaceName)
		if err != nil {
			return err
		}
		bindNetwork, bindAddress, interfaceIndex = network, address, iif.Index
		return control.BindToInterface0(interfaceFinder, conn, network, address, interfaceName, iif.Index, false)
	}
	followFunc := func(conn syscall.Conn) {
		rawConn, _ := conn.SyscallConn()
		l.interfaceCallbacks = append(l.interfaceCallbacks, interfaceFinder.RegisterInterfaceUpdateCallback(func(interfaces []control.Interface) {
			iif := common.Find(interfaces, func(it control.Interface) bool {
				return it.Name == interfaceName
			})
			access.Lock()
			defer access.Unlock()
			if iif.Index == 0 || iif.Index == interfaceIndex {
				return
			}
			err := control.BindToInterface0(interfaceFinder, rawConn, bindNetwork, bindAddress, interfaceName, iif.Index, false)
			if err != nil {
				if !E.IsClosed(err) {
					l.logger.Error(E.Cause(err, "rebind ", N.NetworkName(bindNetwork), " server to interface ", interfaceName))
				}
				return
			}
			interfaceIndex = iif.Index
			l.logger.Info(N.NetworkName(bindNetwork), " server rebound to interface ", interfaceName)
		}))
	}
	return bindFunc, followFunc
}

func (l *Listener) TCPListener() net.Listener {
	return l.tcpListener
}

func (l *Listener) UDPConn() *net.UDPConn {
	return l.udpConn
}

func (l *Listener) ListenOptions() option.ListenOptions {
	return l.listenOptions
}

func ListenNetworkNamespace[T any](ctx context.Context, nameOrPath string, block func() (T, error)) (T, error) {
	if nameOrPath == "" {
		return block()
	}
	manager := service.FromContext[adapter.NetworkNamespaceManager](ctx)
	if manager != nil {
		nameOrPath = manager.ResolvePath(nameOrPath)
	}
	type blockResult struct {
		value T
		err   error
	}
	resultChannel := make(chan blockResult, 1)
	go func() {
		runtime.LockOSThread()
		value, err := listenNetworkNamespaceThread(nameOrPath, block)
		resultChannel <- blockResult{value, err}
	}()
	result := <-resultChannel
	return result.value, result.err
}

func listenNetworkNamespaceThread[T any](nameOrPath string, block func() (T, error)) (T, error) {
	var (
		targetNs netns.NsHandle
		err      error
	)
	if strings.HasPrefix(nameOrPath, "/") {
		targetNs, err = netns.GetFromPath(nameOrPath)
	} else {
		targetNs, err = netns.GetFromName(nameOrPath)
	}
	if err != nil {
		return common.DefaultValue[T](), E.Cause(err, "get netns ", nameOrPath)
	}
	defer targetNs.Close()
	err = netns.Set(targetNs)
	if err != nil {
		return common.DefaultValue[T](), E.Cause(err, "set netns to ", nameOrPath)
	}
	return block()
}
