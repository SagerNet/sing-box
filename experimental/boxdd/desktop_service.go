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
	ownerUserID, err := loadOwner()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if ownerUserID == identity.UserID {
		ownership = DaemonOwnership_DAEMON_OWNERSHIP_CALLER
	} else if ownerUserID != "" {
		ownership = DaemonOwnership_DAEMON_OWNERSHIP_OTHER
	}
	return &DaemonInfo{
		Version:   C.Version,
		Ownership: ownership,
	}, nil
}

func (s *desktopService) InstallUpdate(ctx context.Context, request *InstallUpdateRequest) (*InstallUpdateResponse, error) {
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return s.daemon.installUpdate(identity, request.InstallerPath)
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
	ownerUserID, err := loadOwner()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if ownerUserID != "" && ownerUserID != identity.UserID {
		return nil, status.Error(codes.PermissionDenied, "the service is owned by another user")
	}
	err = s.daemon.preparePlatformOwnerLocked(identity)
	if err != nil {
		return nil, err
	}
	err = saveOwner(identity.UserID, identity.SessionID)
	if err != nil {
		return nil, err
	}
	currentOptions, err := loadStartOptions(identity.UserID)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	mergedOptions := currentOptions
	mergedOptions.WasRunning = true
	if request.Options != nil {
		mergedOptions.OOMKillerEnabled = request.Options.OomKillerEnabled
		mergedOptions.OOMKillerDisabled = request.Options.OomKillerDisabled
		mergedOptions.OOMMemoryLimit = request.Options.OomMemoryLimit
	}
	err = s.daemon.startServiceLocked(ctx, identity.UserID, request.ConfigContent, mergedOptions)
	if err != nil {
		return nil, s.daemon.cleanFailedStartLocked(identity.UserID, mergedOptions, err)
	}
	directory := userWorkingDirectory(identity.UserID)
	configError := atomicfile.WriteFile(filepath.Join(directory, serviceConfigFileName), []byte(request.ConfigContent), 0o600)
	optionsError := saveStartOptions(identity.UserID, mergedOptions)
	if configError != nil || optionsError != nil {
		return nil, s.daemon.cleanFailedStartLocked(identity.UserID, mergedOptions, E.Errors(configError, optionsError))
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
	ownerUserID, err := loadOwner()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if ownerUserID == identity.UserID {
		err = s.daemon.preparePlatformOwnerLocked(identity)
		if err != nil {
			return nil, err
		}
		err = saveOwner(identity.UserID, identity.SessionID)
		if err != nil {
			return nil, err
		}
		return &emptypb.Empty{}, nil
	}
	if ownerUserID != "" {
		return nil, status.Error(codes.Aborted, "the service was claimed by another user")
	}
	err = s.daemon.configureWorkingDirectoryLocked(userWorkingDirectory(identity.UserID))
	if err != nil {
		return nil, err
	}
	err = s.daemon.preparePlatformOwnerLocked(identity)
	if err != nil {
		return nil, err
	}
	err = saveOwner(identity.UserID, identity.SessionID)
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
	if s.daemon.closed {
		s.daemon.lifecycleAccess.Unlock()
		return nil, os.ErrClosed
	}
	ownerUserID, err := loadOwner()
	if err != nil && !os.IsNotExist(err) {
		s.daemon.lifecycleAccess.Unlock()
		return nil, err
	}
	if ownerUserID == "" || ownerUserID == identity.UserID {
		err = s.takeOverServiceLocked(identity, ownerUserID)
		s.daemon.lifecycleAccess.Unlock()
		if err != nil {
			return nil, err
		}
		return &emptypb.Empty{}, nil
	}
	s.daemon.lifecycleAccess.Unlock()
	err = authorizeTakeOver(ctx, identity)
	if err != nil {
		return nil, err
	}
	s.daemon.lifecycleAccess.Lock()
	defer s.daemon.lifecycleAccess.Unlock()
	if s.daemon.closed {
		return nil, os.ErrClosed
	}
	ownerUserID, err = loadOwner()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	err = s.takeOverServiceLocked(identity, ownerUserID)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *desktopService) takeOverServiceLocked(identity peerIdentity, ownerUserID string) error {
	if ownerUserID == identity.UserID {
		err := s.daemon.preparePlatformOwnerLocked(identity)
		if err != nil {
			return err
		}
		return saveOwner(identity.UserID, identity.SessionID)
	}
	if ownerUserID != "" {
		err := s.daemon.stopServiceLocked(ownerUserID)
		if err != nil {
			return err
		}
		if s.daemon.platform != nil {
			err = s.daemon.platform.ReleaseOwner()
			if err != nil {
				return err
			}
		}
	}
	err := s.daemon.configureWorkingDirectoryLocked(userWorkingDirectory(identity.UserID))
	if err != nil {
		return err
	}
	err = s.daemon.preparePlatformOwnerLocked(identity)
	if err != nil {
		return err
	}
	err = saveOwner(identity.UserID, identity.SessionID)
	if err != nil {
		return err
	}
	s.daemon.disconnectPeerConnectionsExcept(identity.UserID)
	return nil
}

func (s *desktopService) GetSecuritySettings(ctx context.Context, empty *emptypb.Empty) (*SecuritySettings, error) {
	_, err := peerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if !insecureModeAvailable() {
		return &SecuritySettings{}, nil
	}
	return &SecuritySettings{
		Available:           true,
		InsecureModeEnabled: s.daemon.insecureModeEnabled(),
	}, nil
}

func (s *desktopService) SetInsecureModeEnabled(ctx context.Context, request *SetInsecureModeEnabledRequest) (*emptypb.Empty, error) {
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if !insecureModeAvailable() {
		return nil, status.Error(codes.FailedPrecondition, "insecure mode is not available on this platform")
	}
	if request.Enabled {
		return nil, status.Error(codes.PermissionDenied, "enabling insecure mode requires an elevated service command")
	}
	s.daemon.lifecycleAccess.Lock()
	defer s.daemon.lifecycleAccess.Unlock()
	if s.daemon.closed {
		return nil, os.ErrClosed
	}
	err = authorizeDisableInsecureMode(identity)
	if err != nil {
		return nil, err
	}
	wasEnabled := s.daemon.insecureModeEnabled()
	err = saveSecuritySettings(workingDirectory, securitySettings{InsecureModeEnabled: false})
	if err != nil {
		return nil, err
	}
	if wasEnabled && s.daemon.startedService.Instance() != nil {
		var ownerUserID string
		ownerUserID, err = loadOwner()
		if err != nil {
			return nil, err
		}
		err = s.daemon.stopServiceLocked(ownerUserID)
		if err != nil {
			return nil, err
		}
	}
	return &emptypb.Empty{}, nil
}

func (d *Daemon) cleanFailedStartLocked(ownerUserID string, options startOptions, startError error) error {
	var platformError error
	if d.platform != nil {
		platformError = d.platform.ResetPlatformOptions()
	}
	if d.startedService.Instance() != nil {
		_ = d.startedService.CloseService()
	}
	directory := userWorkingDirectory(ownerUserID)
	crashReportError := tagUnownedReports(filepath.Join(directory, crashReportsDirectoryName), ownerUserID)
	oomReportError := tagUnownedReports(filepath.Join(directory, oomReportsDirectoryName), ownerUserID)
	options.WasRunning = false
	snapshotError := saveStartOptions(ownerUserID, options)
	return E.Errors(startError, platformError, crashReportError, oomReportError, snapshotError)
}

func (s *desktopService) GetWorkingDirectory(ctx context.Context, empty *emptypb.Empty) (*WorkingDirectoryInfo, error) {
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	s.daemon.lifecycleAccess.Lock()
	defer s.daemon.lifecycleAccess.Unlock()
	ownerUserID, err := loadOwner()
	if err != nil {
		return nil, err
	}
	if ownerUserID != identity.UserID {
		return nil, status.Error(codes.PermissionDenied, "the service is owned by another user")
	}
	directory := userWorkingDirectory(identity.UserID)
	size, err := directorySize(directory)
	if err != nil {
		return nil, err
	}
	return &WorkingDirectoryInfo{
		Path: directory,
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
	ownerUserID, err := loadOwner()
	if err != nil {
		return nil, err
	}
	if ownerUserID != identity.UserID {
		return nil, status.Error(codes.PermissionDenied, "the service is owned by another user")
	}
	directory := userWorkingDirectory(identity.UserID)
	err = s.daemon.configureWorkingDirectoryLocked(workingDirectory)
	if err != nil {
		return nil, err
	}
	err = os.RemoveAll(directory)
	if err != nil {
		restoreError := s.daemon.configureWorkingDirectoryLocked(directory)
		return nil, E.Errors(err, restoreError)
	}
	err = s.daemon.configureWorkingDirectoryLocked(directory)
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
