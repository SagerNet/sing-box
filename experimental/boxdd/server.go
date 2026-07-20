package main

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/experimental/libbox"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/service/oomkiller"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type Daemon struct {
	ctx                     context.Context
	logger                  log.ContextLogger
	startedService          *daemon.StartedService
	server                  *grpc.Server
	runtimeWorkingDirectory string
	lifecycleAccess         sync.Mutex
	closed                  bool
	peerAccess              sync.Mutex
	peerConnections         map[peerConnection]peerIdentity
	platform                daemonPlatform
}

func newDaemon() (*Daemon, error) {
	ctx := include.Context(context.Background())
	d := &Daemon{
		ctx:                     ctx,
		logger:                  log.StdLogger(),
		runtimeWorkingDirectory: workingDirectory,
	}
	platformInterface, err := newPlatformInterface(d)
	if err != nil {
		return nil, err
	}
	d.platform = platformInterface
	if platformInterface != nil {
		service.MustRegister[adapter.PlatformInterface](ctx, platformInterface)
	}
	registerSecurityPolicy(ctx, d)
	d.startedService = daemon.NewStartedService(daemon.ServiceOptions{
		Context:     ctx,
		LogMaxLines: 3000,
	})
	reporter := libbox.NewOOMReporter(d.startedService)
	service.MustRegister[oomkiller.OOMReporter](ctx, reporter)
	managedService := daemon.NewManagedService(daemon.ManagedServiceOptions{
		Handler:     &managedHandler{d},
		Debug:       debugEnabled,
		OOMReporter: reporter,
	})
	authorizer := newAuthorizer(d)
	serverOptions := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(newUnaryAuthorizeInterceptor(authorizer), daemon.UnaryLocaleInterceptor),
		grpc.ChainStreamInterceptor(newStreamAuthorizeInterceptor(authorizer), daemon.StreamLocaleInterceptor),
	}
	platformOptions, err := platformServerOptions(d)
	if err != nil {
		return nil, err
	}
	serverOptions = append(serverOptions, platformOptions...)
	d.server = grpc.NewServer(serverOptions...)
	daemon.RegisterStartedServiceServer(d.server, d.startedService)
	daemon.RegisterManagedServiceServer(d.server, managedService)
	RegisterDesktopServiceServer(d.server, &desktopService{daemon: d})
	healthServer := health.NewServer()
	healthServer.SetServingStatus(daemon.StartedService_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(daemon.ManagedService_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(DesktopService_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(d.server, healthServer)
	if listenAddress != "" {
		reflection.Register(d.server)
	}
	return d, nil
}

func (d *Daemon) listen() (net.Listener, error) {
	if listenAddress != "" {
		d.logger.Warn("listening on TCP address ", listenAddress, ": development only, no access control")
		return net.Listen("tcp", listenAddress)
	}
	return listenEndpoint()
}

func (d *Daemon) Start() error {
	listener, err := d.listen()
	if err != nil {
		return err
	}
	d.logger.Info("daemon listening at ", listener.Addr())
	go func() {
		serveError := d.server.Serve(listener)
		if serveError != nil && !errors.Is(serveError, grpc.ErrServerStopped) {
			d.logger.Error("serve: ", serveError)
		}
	}()
	go d.restore()
	return nil
}

func (d *Daemon) restore() {
	d.lifecycleAccess.Lock()
	defer d.lifecycleAccess.Unlock()
	if d.closed {
		return
	}
	ownerState, err := loadOwnerState()
	if err != nil {
		if !os.IsNotExist(err) {
			d.logger.Warn("load owner: ", err)
		}
		return
	}
	ownerUserID := ownerState.UserID
	ownerWorkingDirectory := userWorkingDirectory(ownerUserID)
	err = d.configureWorkingDirectoryLocked(ownerWorkingDirectory)
	if err != nil {
		d.logger.Warn("configure working directory: ", err)
		return
	}
	options, err := loadStartOptions(ownerUserID)
	if err != nil {
		if !os.IsNotExist(err) {
			d.logger.Warn("load start options: ", err)
		}
		return
	}
	err = tagUnownedReports(filepath.Join(ownerWorkingDirectory, crashReportsDirectoryName), ownerUserID)
	if err != nil {
		d.logger.Warn("tag crash reports: ", err)
	}
	err = tagUnownedReports(filepath.Join(ownerWorkingDirectory, oomReportsDirectoryName), ownerUserID)
	if err != nil {
		d.logger.Warn("tag OOM reports: ", err)
	}
	if !options.WasRunning {
		return
	}
	if d.platform != nil {
		d.platform.SetSystemProxyPreference(options.systemProxyEnabled())
		err = d.platform.RestoreOwner(ownerState)
		if err != nil {
			d.logger.Warn("restore owner session: ", err)
		}
	}
	configContent, err := loadServiceConfig(ownerUserID)
	if err != nil {
		d.logger.Error("restore service: ", err)
		return
	}
	d.logger.Info("restoring service")
	err = d.startServiceLocked(d.ctx, ownerUserID, configContent, options)
	if err != nil {
		d.logger.Error("restore service: ", err)
	}
}

func (d *Daemon) configureWorkingDirectoryLocked(directory string) error {
	if d.runtimeWorkingDirectory == directory {
		return nil
	}
	err := os.MkdirAll(directory, 0o700)
	if err != nil {
		return err
	}
	err = os.Chdir(directory)
	if err != nil {
		return err
	}
	err = libbox.Setup(&libbox.SetupOptions{
		BasePath:          directory,
		WorkingPath:       directory,
		TempPath:          directory,
		CrashReportSource: "Daemon",
	})
	if err != nil {
		return err
	}
	libbox.PromoteOOMDraft()
	d.runtimeWorkingDirectory = directory
	return nil
}

func (d *Daemon) startServiceLocked(ctx context.Context, ownerUserID string, configContent string, options startOptions) error {
	directory := userWorkingDirectory(ownerUserID)
	err := d.configureWorkingDirectoryLocked(directory)
	if err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(directory, configSnapshotFileName), []byte(configContent), 0o600)
	libbox.ReloadSetupOptions(&libbox.SetupOptions{
		OomKillerEnabled:  options.OOMKillerEnabled,
		OomKillerDisabled: options.OOMKillerDisabled,
		OomMemoryLimit:    options.OOMMemoryLimit,
	})
	d.startedService.SetOOMKillerOptions(options.OOMKillerEnabled, options.OOMKillerDisabled, uint64(options.OOMMemoryLimit))
	if d.platform != nil {
		d.platform.SetSystemProxyPreference(options.systemProxyEnabled())
		err = d.platform.ResetPlatformOptions()
		if err != nil {
			return err
		}
	}
	err = d.startedService.StartOrReloadService(ctx, configContent, nil)
	if err != nil && d.platform != nil {
		return E.Errors(err, d.platform.ResetPlatformOptions())
	}
	return err
}

func (d *Daemon) stopServiceLocked(ownerUserID string) error {
	options, err := loadStartOptions(ownerUserID)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if d.platform != nil {
		err = d.platform.ResetPlatformOptions()
		if err != nil {
			return err
		}
	}
	if d.startedService.Instance() != nil {
		err = d.startedService.CloseService()
		if err != nil {
			return err
		}
	}
	directory := userWorkingDirectory(ownerUserID)
	crashReportError := tagUnownedReports(filepath.Join(directory, crashReportsDirectoryName), ownerUserID)
	if crashReportError != nil {
		return crashReportError
	}
	oomReportError := tagUnownedReports(filepath.Join(directory, oomReportsDirectoryName), ownerUserID)
	if oomReportError != nil {
		return oomReportError
	}
	options.WasRunning = false
	return saveStartOptions(ownerUserID, options)
}

func (d *Daemon) Close() {
	d.lifecycleAccess.Lock()
	d.closed = true
	d.lifecycleAccess.Unlock()
	d.server.Stop()
	d.lifecycleAccess.Lock()
	if d.platform != nil {
		_ = d.platform.ResetPlatformOptions()
	}
	_ = d.startedService.CloseService()
	d.startedService.Close()
	if d.platform != nil {
		_ = d.platform.Close()
	}
	d.lifecycleAccess.Unlock()
}

func (d *Daemon) disconnectPeerConnectionsExcept(userID string) {
	d.peerAccess.Lock()
	var connections []peerConnection
	for connection, identity := range d.peerConnections {
		if identity.UserID != userID {
			connections = append(connections, connection)
		}
	}
	d.peerAccess.Unlock()
	for _, connection := range connections {
		connection.Close()
	}
}

type Authorizer interface {
	Authorize(ctx context.Context, method string) error
	InvokeUnary(ctx context.Context, method string, handler func() (any, error)) (any, error)
}

func newAuthorizer(daemon *Daemon) Authorizer {
	if listenAddress != "" {
		return &allowAllAuthorizer{daemon: daemon}
	}
	return &daemonAuthorizer{daemon: daemon}
}

type allowAllAuthorizer struct {
	daemon *Daemon
}

func (a *allowAllAuthorizer) Authorize(ctx context.Context, method string) error {
	return nil
}

func (a *allowAllAuthorizer) InvokeUnary(ctx context.Context, method string, handler func() (any, error)) (any, error) {
	if ownerProtectedMethod(method) {
		a.daemon.lifecycleAccess.Lock()
		defer a.daemon.lifecycleAccess.Unlock()
	}
	return handler()
}

type daemonAuthorizer struct {
	daemon *Daemon
}

func (a *daemonAuthorizer) Authorize(ctx context.Context, method string) error {
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}
	desktopPrefix := "/" + DesktopService_ServiceDesc.ServiceName + "/"
	if strings.HasPrefix(method, desktopPrefix) {
		return nil
	}
	if ownerProtectedMethod(method) {
		a.daemon.lifecycleAccess.Lock()
		defer a.daemon.lifecycleAccess.Unlock()
		err = a.daemon.authorizeOwnerLocked(identity.UserID)
		if err != nil {
			return err
		}
		err = a.daemon.preparePlatformOwnerLocked(identity)
		if err != nil {
			return err
		}
		if a.daemon.platform != nil {
			return saveOwner(identity.UserID, identity.SessionID)
		}
		return nil
	}
	return status.Error(codes.PermissionDenied, "the service is not available")
}

func (a *daemonAuthorizer) InvokeUnary(ctx context.Context, method string, handler func() (any, error)) (any, error) {
	if !ownerProtectedMethod(method) {
		err := a.Authorize(ctx, method)
		if err != nil {
			return nil, err
		}
		return handler()
	}
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	a.daemon.lifecycleAccess.Lock()
	defer a.daemon.lifecycleAccess.Unlock()
	err = a.daemon.authorizeOwnerLocked(identity.UserID)
	if err != nil {
		return nil, err
	}
	err = a.daemon.preparePlatformOwnerLocked(identity)
	if err != nil {
		return nil, err
	}
	if a.daemon.platform != nil {
		err = saveOwner(identity.UserID, identity.SessionID)
		if err != nil {
			return nil, err
		}
	}
	return handler()
}

func ownerProtectedMethod(method string) bool {
	startedPrefix := "/" + daemon.StartedService_ServiceDesc.ServiceName + "/"
	managedPrefix := "/" + daemon.ManagedService_ServiceDesc.ServiceName + "/"
	return strings.HasPrefix(method, startedPrefix) || strings.HasPrefix(method, managedPrefix)
}

func (d *Daemon) authorizeOwnerLocked(userID string) error {
	ownerUserID, err := loadOwner()
	if err != nil {
		if os.IsNotExist(err) {
			return status.Error(codes.PermissionDenied, "the service has no owner")
		}
		return err
	}
	if ownerUserID == "" || ownerUserID != userID {
		return status.Error(codes.PermissionDenied, "the service is owned by another user")
	}
	return nil
}

func newUnaryAuthorizeInterceptor(authorizer Authorizer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return authorizer.InvokeUnary(ctx, info.FullMethod, func() (any, error) {
			return handler(ctx, request)
		})
	}
}

func newStreamAuthorizeInterceptor(authorizer Authorizer) grpc.StreamServerInterceptor {
	return func(server any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := authorizer.Authorize(stream.Context(), info.FullMethod)
		if err != nil {
			return err
		}
		return handler(server, stream)
	}
}
