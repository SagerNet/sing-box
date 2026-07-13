package main

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/daemon"
)

type daemonPlatform interface {
	adapter.PlatformInterface
	PrepareOwner(identity peerIdentity) error
	RestoreOwner(state ownerState) error
	ReleaseOwner() error
	ResetPlatformOptions() error
	SetSystemProxyPreference(enabled bool)
	SystemProxyStatus() (*daemon.SystemProxyStatus, error)
	SetSystemProxyEnabled(enabled bool) error
	HandleSessionChange(eventType uint32, sessionID uint32, state ownerState) (uint32, bool, error)
	Close() error
}

func (d *Daemon) preparePlatformOwnerLocked(identity peerIdentity) error {
	if d.platform == nil {
		return nil
	}
	return d.platform.PrepareOwner(identity)
}
