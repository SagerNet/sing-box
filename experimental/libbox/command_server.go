package libbox

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type CommandServer struct {
	*daemon.StartedService
	handler           CommandServerHandler
	platformInterface PlatformInterface
	platformWrapper   *platformInterfaceWrapper
	grpcServer        *grpc.Server
	listener          net.Listener
	endPauseTimer     *time.Timer
}

type CommandServerHandler interface {
	ServiceStop() error
	ServiceReload() error
	GetSystemProxyStatus() (*SystemProxyStatus, error)
	SetSystemProxyEnabled(enabled bool) error
	WriteDebugMessage(message string)
}

func NewCommandServer(handler CommandServerHandler, platformInterface PlatformInterface) (*CommandServer, error) {
	ctx := BaseContext(platformInterface)
	platformWrapper := &platformInterfaceWrapper{
		iif:       platformInterface,
		useProcFS: platformInterface.UseProcFS(),
	}
	service.MustRegister[adapter.PlatformInterface](ctx, platformWrapper)
	server := &CommandServer{
		handler:           handler,
		platformInterface: platformInterface,
		platformWrapper:   platformWrapper,
	}
	server.StartedService = daemon.NewStartedService(daemon.ServiceOptions{
		Context: ctx,
		// Platform:         platformWrapper,
		Handler:     (*platformHandler)(server),
		Debug:       sDebug,
		LogMaxLines: sLogMaxLines,
		// WorkingDirectory: sWorkingPath,
		// TempDirectory:    sTempPath,
		// UserID:           sUserID,
		// GroupID:          sGroupID,
		// SystemProxyEnabled: false,
	})
	return server, nil
}

func unaryAuthInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if sCommandServerSecret == "" {
		return handler(ctx, req)
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	values := md.Get("x-command-secret")
	if len(values) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authentication secret")
	}
	if values[0] != sCommandServerSecret {
		return nil, status.Error(codes.Unauthenticated, "invalid authentication secret")
	}
	return handler(ctx, req)
}

func streamAuthInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if sCommandServerSecret == "" {
		return handler(srv, ss)
	}
	md, ok := metadata.FromIncomingContext(ss.Context())
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}
	values := md.Get("x-command-secret")
	if len(values) == 0 {
		return status.Error(codes.Unauthenticated, "missing authentication secret")
	}
	if values[0] != sCommandServerSecret {
		return status.Error(codes.Unauthenticated, "invalid authentication secret")
	}
	return handler(srv, ss)
}

func (s *CommandServer) Start() error {
	var (
		listener net.Listener
		err      error
	)
	if sCommandServerListenPort == 0 {
		sockPath := filepath.Join(sBasePath, "command.sock")
		os.Remove(sockPath)
		for i := 0; i < 30; i++ {
			listener, err = net.ListenUnix("unix", &net.UnixAddr{
				Name: sockPath,
				Net:  "unix",
			})
			if err == nil {
				break
			}
			if !errors.Is(err, syscall.EROFS) {
				break
			}
			time.Sleep(time.Second)
		}
		if err != nil {
			return E.Cause(err, "listen command server")
		}
		if sUserID != os.Getuid() {
			err = os.Chown(sockPath, sUserID, sGroupID)
			if err != nil {
				listener.Close()
				os.Remove(sockPath)
				return E.Cause(err, "chown")
			}
		}
	} else {
		listener, err = net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(sCommandServerListenPort))))
		if err != nil {
			return E.Cause(err, "listen command server")
		}
	}
	s.listener = listener
	serverOptions := []grpc.ServerOption{
		grpc.UnaryInterceptor(unaryAuthInterceptor),
		grpc.StreamInterceptor(streamAuthInterceptor),
	}
	s.grpcServer = grpc.NewServer(serverOptions...)
	daemon.RegisterStartedServiceServer(s.grpcServer, s.StartedService)
	go s.grpcServer.Serve(listener)
	return nil
}

func (s *CommandServer) Close() {
	if s.grpcServer != nil {
		s.grpcServer.Stop()
	}
	common.Close(s.listener)
}

type OverrideOptions struct {
	AutoRedirect   bool
	IncludePackage StringIterator
	ExcludePackage StringIterator
}

func (s *CommandServer) StartOrReloadService(configContent string, options *OverrideOptions) error {
	return s.StartedService.StartOrReloadService(configContent, &daemon.OverrideOptions{
		AutoRedirect:   options.AutoRedirect,
		IncludePackage: iteratorToArray(options.IncludePackage),
		ExcludePackage: iteratorToArray(options.ExcludePackage),
	})
}

func (s *CommandServer) CloseService() error {
	return s.StartedService.CloseService()
}

func (s *CommandServer) WriteMessage(level int32, message string) {
	s.StartedService.WriteMessage(log.Level(level), message)
}

func (s *CommandServer) SetError(message string) {
	s.StartedService.SetError(E.New(message))
}

func (s *CommandServer) NeedWIFIState() bool {
	instance := s.StartedService.Instance()
	if instance == nil || instance.Box() == nil {
		return false
	}
	return instance.Box().Network().NeedWIFIState()
}

func (s *CommandServer) NeedFindProcess() bool {
	instance := s.StartedService.Instance()
	if instance == nil || instance.Box() == nil {
		return false
	}
	return instance.Box().Router().NeedFindProcess()
}

func (s *CommandServer) Pause() {
	instance := s.StartedService.Instance()
	if instance == nil || instance.PauseManager() == nil {
		return
	}
	instance.PauseManager().DevicePause()
	if C.IsIos {
		if s.endPauseTimer == nil {
			s.endPauseTimer = time.AfterFunc(time.Minute, instance.PauseManager().DeviceWake)
		} else {
			s.endPauseTimer.Reset(time.Minute)
		}
	}
}

func (s *CommandServer) Wake() {
	instance := s.StartedService.Instance()
	if instance == nil || instance.PauseManager() == nil {
		return
	}
	if !C.IsIos {
		instance.PauseManager().DeviceWake()
	}
}

func (s *CommandServer) ResetNetwork() {
	instance := s.StartedService.Instance()
	if instance == nil || instance.Box() == nil {
		return
	}
	instance.Box().Router().ResetNetwork()
}

func (s *CommandServer) UpdateWIFIState() {
	instance := s.StartedService.Instance()
	if instance == nil || instance.Box() == nil {
		return
	}
	instance.Box().Network().UpdateWIFIState()
}

type platformHandler CommandServer

func (h *platformHandler) ServiceStop() error {
	return (*CommandServer)(h).handler.ServiceStop()
}

func (h *platformHandler) ServiceReload() error {
	return (*CommandServer)(h).handler.ServiceReload()
}

func (h *platformHandler) SystemProxyStatus() (*daemon.SystemProxyStatus, error) {
	status, err := (*CommandServer)(h).handler.GetSystemProxyStatus()
	if err != nil {
		return nil, err
	}
	return &daemon.SystemProxyStatus{
		Enabled:   status.Enabled,
		Available: status.Available,
	}, nil
}

func (h *platformHandler) SetSystemProxyEnabled(enabled bool) error {
	return (*CommandServer)(h).handler.SetSystemProxyEnabled(enabled)
}

func (h *platformHandler) WriteDebugMessage(message string) {
	(*CommandServer)(h).handler.WriteDebugMessage(message)
}
