//go:build windows

package vboxusb

import (
	"sync"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

// EnableLoadDriverPrivilege flips SeLoadDriverPrivilege ON in the
// current process token. Required to open \\.\VBoxUSBMon and to drive
// PnP install/uninstall flows; non-interactive services have it
// disabled by default. Idempotent.
func EnableLoadDriverPrivilege() error {
	loadDriverPrivOnce.Do(func() {
		loadDriverPrivErr = enableLoadDriverPrivilege()
	})
	return loadDriverPrivErr
}

var (
	loadDriverPrivOnce sync.Once
	loadDriverPrivErr  error
)

func enableLoadDriverPrivilege() error {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token)
	if err != nil {
		return E.Cause(err, "vboxusb: open process token")
	}
	defer token.Close()

	privName, _ := windows.UTF16PtrFromString("SeLoadDriverPrivilege")
	var luid windows.LUID
	err = windows.LookupPrivilegeValue(nil, privName, &luid)
	if err != nil {
		return E.Cause(err, "vboxusb: lookup SeLoadDriverPrivilege")
	}

	tp := windows.Tokenprivileges{
		PrivilegeCount: 1,
		Privileges: [1]windows.LUIDAndAttributes{{
			Luid:       luid,
			Attributes: windows.SE_PRIVILEGE_ENABLED,
		}},
	}
	err = windows.AdjustTokenPrivileges(token, false, &tp, 0, nil, nil)
	if err != nil {
		return E.Cause(err, "vboxusb: adjust token privileges")
	}
	return nil
}
