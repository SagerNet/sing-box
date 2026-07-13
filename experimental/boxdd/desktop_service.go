package main

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/tailscale/atomicfile"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var _ DesktopServiceServer = (*desktopService)(nil)

type desktopService struct {
	UnimplementedDesktopServiceServer
	daemon *Daemon
}

func (s *desktopService) GetDaemonInfo(ctx context.Context, empty *emptypb.Empty) (*DaemonInfo, error) {
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	ownership := DaemonOwnership_DAEMON_OWNERSHIP_AVAILABLE
	options, err := loadStartOptions()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if options.OwnerUserID == identity.UserID {
		ownership = DaemonOwnership_DAEMON_OWNERSHIP_CALLER
	} else if options.OwnerUserID != "" {
		ownership = DaemonOwnership_DAEMON_OWNERSHIP_OTHER
	}
	return &DaemonInfo{
		Version:   C.Version,
		Ownership: ownership,
	}, nil
}

func (s *desktopService) StartService(ctx context.Context, request *StartServiceRequest) (*emptypb.Empty, error) {
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	s.daemon.lifecycleAccess.Lock()
	defer s.daemon.lifecycleAccess.Unlock()
	if s.daemon.closed {
		return nil, os.ErrClosed
	}
	currentOptions, err := loadStartOptions()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if currentOptions.OwnerUserID != "" && currentOptions.OwnerUserID != identity.UserID {
		return nil, status.Error(codes.PermissionDenied, "the service is owned by another user")
	}
	mergedOptions := currentOptions
	mergedOptions.WasRunning = true
	mergedOptions.OwnerUserID = identity.UserID
	if request.Options != nil {
		mergedOptions.OOMKillerEnabled = request.Options.OomKillerEnabled
		mergedOptions.OOMKillerDisabled = request.Options.OomKillerDisabled
		mergedOptions.OOMMemoryLimit = request.Options.OomMemoryLimit
	}
	err = s.daemon.startService(request.ConfigContent, mergedOptions)
	if err != nil {
		return nil, s.daemon.cleanFailedStartLocked(identity.UserID, err)
	}
	configError := atomicfile.WriteFile(filepath.Join(workingDirectory, serviceConfigFileName), []byte(request.ConfigContent), 0o600)
	optionsError := saveStartOptions(mergedOptions)
	if configError != nil || optionsError != nil {
		return nil, s.daemon.cleanFailedStartLocked(identity.UserID, E.Errors(configError, optionsError))
	}
	return &emptypb.Empty{}, nil
}

func (s *desktopService) ClaimService(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	s.daemon.lifecycleAccess.Lock()
	defer s.daemon.lifecycleAccess.Unlock()
	if s.daemon.closed {
		return nil, os.ErrClosed
	}
	options, err := loadStartOptions()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if options.OwnerUserID == identity.UserID {
		return &emptypb.Empty{}, nil
	}
	if options.OwnerUserID != "" {
		return nil, status.Error(codes.Aborted, "the service was claimed by another user")
	}
	err = s.daemon.resetRuntimeOwnerLocked(identity.UserID)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *desktopService) TakeOverService(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	s.daemon.lifecycleAccess.Lock()
	defer s.daemon.lifecycleAccess.Unlock()
	if s.daemon.closed {
		return nil, os.ErrClosed
	}
	options, err := loadStartOptions()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if options.OwnerUserID == identity.UserID {
		return &emptypb.Empty{}, nil
	}
	err = s.daemon.stopServiceLocked(identity.UserID)
	if err != nil {
		return nil, err
	}
	s.daemon.disconnectPeerConnectionsExcept(identity.UserID)
	return &emptypb.Empty{}, nil
}

func (d *Daemon) cleanFailedStartLocked(ownerUserID string, startError error) error {
	closeError := d.startedService.CloseService()
	crashReportError := tagUnownedReports(filepath.Join(workingDirectory, crashReportsDirectoryName), ownerUserID)
	oomReportError := tagUnownedReports(filepath.Join(workingDirectory, oomReportsDirectoryName), ownerUserID)
	resetError := d.resetRuntimeOwnerLocked(ownerUserID)
	return E.Errors(startError, closeError, crashReportError, oomReportError, resetError)
}

func (s *desktopService) GetWorkingDirectory(ctx context.Context, empty *emptypb.Empty) (*WorkingDirectoryInfo, error) {
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	s.daemon.lifecycleAccess.Lock()
	defer s.daemon.lifecycleAccess.Unlock()
	options, err := loadStartOptions()
	if err != nil {
		return nil, err
	}
	if options.OwnerUserID != identity.UserID {
		return nil, status.Error(codes.PermissionDenied, "the service is owned by another user")
	}
	size, err := directorySize(workingDirectory)
	if err != nil {
		return nil, err
	}
	return &WorkingDirectoryInfo{
		Path: workingDirectory,
		Size: size,
	}, nil
}

func (s *desktopService) DestroyWorkingDirectory(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	s.daemon.lifecycleAccess.Lock()
	defer s.daemon.lifecycleAccess.Unlock()
	if s.daemon.closed {
		return nil, os.ErrClosed
	}
	if s.daemon.startedService.Instance() != nil {
		return nil, status.Error(codes.FailedPrecondition, "the service must be stopped before destroying the working directory")
	}
	options, err := loadStartOptions()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if options.OwnerUserID != "" && options.OwnerUserID != identity.UserID {
		return nil, status.Error(codes.PermissionDenied, "the service is owned by another user")
	}
	err = s.daemon.resetRuntimeOwnerLocked(identity.UserID)
	if err != nil {
		return nil, err
	}
	err = deleteReportsForUser(filepath.Join(workingDirectory, crashReportsDirectoryName), identity.UserID)
	if err != nil {
		return nil, err
	}
	err = deleteReportsForUser(filepath.Join(workingDirectory, oomReportsDirectoryName), identity.UserID)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func directorySize(root string) (int64, error) {
	var size int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		size += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return size, nil
}
