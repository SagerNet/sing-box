package main

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/daemon"
	E "github.com/sagernet/sing/common/exceptions"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ daemon.ManagedHandler = (*managedHandler)(nil)

type managedHandler struct {
	daemon *Daemon
}

func (h *managedHandler) ServiceStop() error {
	if h.daemon.closed {
		return os.ErrClosed
	}
	ownerUserID, err := loadOwner()
	if err != nil {
		return err
	}
	return h.daemon.stopServiceLocked(ownerUserID)
}

func (h *managedHandler) ServiceReload(ctx context.Context) error {
	if h.daemon.closed {
		return os.ErrClosed
	}
	ownerUserID, err := loadOwner()
	if err != nil {
		return err
	}
	configContent, err := loadServiceConfig(ownerUserID)
	if err != nil {
		return err
	}
	options, err := loadStartOptions(ownerUserID)
	if err != nil {
		return err
	}
	err = h.daemon.startServiceLocked(ctx, ownerUserID, configContent, options)
	if err != nil {
		return err
	}
	options.WasRunning = true
	return saveStartOptions(ownerUserID, options)
}

func (h *managedHandler) SystemProxyStatus() (*daemon.SystemProxyStatus, error) {
	if h.daemon.platform == nil {
		return &daemon.SystemProxyStatus{}, nil
	}
	return h.daemon.platform.SystemProxyStatus()
}

func (h *managedHandler) SetSystemProxyEnabled(enabled bool) error {
	if h.daemon.platform == nil {
		if !enabled {
			return nil
		}
		return status.Error(codes.FailedPrecondition, "the system proxy is not available")
	}
	ownerUserID, err := loadOwner()
	if err != nil {
		return err
	}
	options, err := loadStartOptions(ownerUserID)
	if err != nil {
		return err
	}
	previousEnabled := options.systemProxyEnabled()
	err = h.daemon.platform.SetSystemProxyEnabled(enabled)
	if err != nil {
		return err
	}
	options.SystemProxyEnabled = &enabled
	err = saveStartOptions(ownerUserID, options)
	if err != nil {
		rollbackError := h.daemon.platform.SetSystemProxyEnabled(previousEnabled)
		return E.Errors(err, rollbackError)
	}
	return nil
}

func (h *managedHandler) TriggerNativeCrash() error {
	return E.New("native crash is not supported")
}
