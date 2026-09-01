//go:build linux

package libbox

import (
	"context"
	"net"
	"net/netip"
	"os"
	"runtime"
	"sync"

	"github.com/sagernet/sing-box/common/srs"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewAutoRedirectService(options []byte, handler AutoRedirectHandler) (AutoRedirectSession, error) {
	tunOptions, tableName, redirectPort, err := decodeAutoRedirectOptions(options)
	if err != nil {
		return nil, err
	}
	tunOptions.AutoRedirectMarkMode = true
	autoRedirectLogger := &autoRedirectServiceLogger{handler}
	tunOptions.Logger = autoRedirectLogger
	session := &autoRedirectServiceSession{}
	session.close = sync.OnceValue(func() error {
		return common.Close(session.autoRedirect, session.networkMonitor)
	})
	networkMonitor, err := tun.NewNetworkUpdateMonitor(autoRedirectLogger)
	if err != nil {
		_ = session.Close()
		return nil, E.Cause(err, "create network monitor")
	}
	session.networkMonitor = networkMonitor
	err = networkMonitor.Start()
	if err != nil {
		_ = session.Close()
		return nil, E.Cause(err, "start network monitor")
	}
	autoRedirect, err := tun.NewAutoRedirect(tun.AutoRedirectOptions{
		TunOptions:         tunOptions,
		Context:            context.Background(),
		Handler:            &autoRedirectServiceHandler{handler: handler, logger: autoRedirectLogger},
		Logger:             autoRedirectLogger,
		NetworkMonitor:     networkMonitor,
		InterfaceFinder:    control.NewDefaultInterfaceFinder(),
		TableName:          tableName,
		CustomRedirectPort: func() int { return int(redirectPort) },
		CustomRedirectListenerFD: func() (int, error) {
			fd, fdErr := handler.RedirectListenerFileDescriptor()
			return int(fd), fdErr
		},
		RouteAddressSet: func() ([]netip.Prefix, []netip.Prefix, error) {
			fd, fdErr := handler.RouteAddressSetFileDescriptor()
			if fdErr != nil {
				return nil, nil, fdErr
			}
			file := os.NewFile(uintptr(fd), "route-address-set")
			defer file.Close()
			return srs.ReadRouteAddressSet(file)
		},
		AndroidVPNService: runtime.GOOS == "android",
	})
	if err != nil {
		_ = session.Close()
		return nil, E.Cause(err, "initialize auto-redirect")
	}
	err = autoRedirect.Start()
	if err != nil {
		_ = session.Close()
		return nil, E.Cause(err, "start auto-redirect")
	}
	session.autoRedirect = autoRedirect
	return session, nil
}

type autoRedirectServiceSession struct {
	autoRedirect   tun.AutoRedirect
	networkMonitor tun.NetworkUpdateMonitor
	close          func() error
}

func (s *autoRedirectServiceSession) Close() error {
	return s.close()
}

func (s *autoRedirectServiceSession) UpdateRouteAddressSet() error {
	return s.autoRedirect.UpdateRouteAddressSet()
}

type autoRedirectServiceHandler struct {
	handler AutoRedirectHandler
	logger  logger.Logger
}

func (h *autoRedirectServiceHandler) JudgeFlow(network uint8, source netip.AddrPort, destination netip.AddrPort, firstPacket []byte) tun.FlowVerdict {
	action, err := h.handler.JudgeFlow(int32(network), source.Addr().String(), int32(source.Port()), destination.Addr().String(), int32(destination.Port()), firstPacket)
	if err != nil {
		h.logger.Error("judge flow ", source, " -> ", destination, ": ", err)
		return tun.FlowVerdict{Action: tun.ActionAccept}
	}
	return tun.FlowVerdict{Action: tun.FlowAction(action)}
}

func (h *autoRedirectServiceHandler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	_ = conn.Close()
}

type autoRedirectServiceLogger struct {
	handler AutoRedirectHandler
}

func (l *autoRedirectServiceLogger) Trace(args ...any) {
	l.handler.WriteLog(int32(log.LevelTrace), F.ToString(args...))
}

func (l *autoRedirectServiceLogger) Debug(args ...any) {
	l.handler.WriteLog(int32(log.LevelDebug), F.ToString(args...))
}

func (l *autoRedirectServiceLogger) Info(args ...any) {
	l.handler.WriteLog(int32(log.LevelInfo), F.ToString(args...))
}

func (l *autoRedirectServiceLogger) Warn(args ...any) {
	l.handler.WriteLog(int32(log.LevelWarn), F.ToString(args...))
}

func (l *autoRedirectServiceLogger) Error(args ...any) {
	l.handler.WriteLog(int32(log.LevelError), F.ToString(args...))
}

func (l *autoRedirectServiceLogger) Fatal(args ...any) {
	l.handler.WriteLog(int32(log.LevelFatal), F.ToString(args...))
}

func (l *autoRedirectServiceLogger) Panic(args ...any) {
	l.handler.WriteLog(int32(log.LevelPanic), F.ToString(args...))
}

var (
	_ AutoRedirectSession     = (*autoRedirectServiceSession)(nil)
	_ tun.AutoRedirectHandler = (*autoRedirectServiceHandler)(nil)
	_ logger.Logger           = (*autoRedirectServiceLogger)(nil)
)
