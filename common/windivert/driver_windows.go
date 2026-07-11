//go:build windows

package windivert

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

const (
	driverServiceName = "WinDivert"
	driverDeviceName  = `\\.\WinDivert`
)

// driverDevName is ASCII-safe and must be available before installDriver
// so Open can try CreateFile first and only install on FILE_NOT_FOUND.
var driverDevName, _ = windows.UTF16PtrFromString(driverDeviceName)

// Requires SeLoadDriverPrivilege (Administrator). Running the 386 build
// under WOW64 on a 64-bit kernel is rejected — use the amd64 build.
func installDriver() error {
	if runtime.GOARCH == "386" {
		var isWow64 bool
		err := windows.IsWow64Process(windows.CurrentProcess(), &isWow64)
		if err == nil && isWow64 {
			return E.New("windivert: 386 build detected running under WOW64 on a 64-bit kernel; use the amd64 build")
		}
	}

	// Serialize driver install across concurrent processes. CreateMutex
	// hands back a valid handle together with ERROR_ALREADY_EXISTS when
	// another install already created the mutex.
	mutexName, _ := windows.UTF16PtrFromString("WinDivertDriverInstallMutex")
	mutex, err := windows.CreateMutex(nil, false, mutexName)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return E.Cause(err, "windivert: create install mutex")
	}
	defer windows.CloseHandle(mutex)
	_, err = windows.WaitForSingleObject(mutex, windows.INFINITE)
	if err != nil {
		return E.Cause(err, "windivert: wait install mutex")
	}
	defer windows.ReleaseMutex(mutex)

	sysPath, sysFile, err := extractVerified()
	if err != nil {
		return err
	}
	defer sysFile.Close()
	sysPathW, err := windows.UTF16PtrFromString(sysPath)
	if err != nil {
		return E.Cause(err, "windivert: utf16 driver path")
	}

	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_ALL_ACCESS)
	if err != nil {
		return E.Cause(err, "windivert: open SCM")
	}
	defer windows.CloseServiceHandle(manager)

	serviceNameW, _ := windows.UTF16PtrFromString(driverServiceName)
	// A stopped service record marked for deletion lingers while any handle
	// keeps it alive — including the one OpenService just returned to us.
	// StartService on it reports ERROR_SERVICE_DISABLED, and
	// ChangeServiceConfig cannot un-doom it (ERROR_SERVICE_MARKED_FOR_DELETE).
	// The only way out is to close every handle so SCM drops the record,
	// then create it anew.
	for attempt := 0; ; attempt++ {
		err = tryInstallService(manager, serviceNameW, sysPathW)
		if err == nil {
			return nil
		}
		retryable := errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) ||
			errors.Is(err, windows.ERROR_SERVICE_DISABLED)
		if !retryable || attempt >= 20 {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func tryInstallService(manager windows.Handle, serviceNameW, sysPathW *uint16) error {
	service, err := openOrCreateService(manager, serviceNameW, sysPathW)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(service)

	err = windows.StartService(service, 0, nil)
	if err == nil {
		// Mark for deletion so the driver unregisters when the last handle
		// closes or on next reboot. Matches the upstream DLL's behavior:
		// only the process that actually started the service takes on the
		// cleanup responsibility. If another process already started it,
		// we leave DeleteService to them.
		_ = windows.DeleteService(service)
		return nil
	}
	if errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return nil
	}
	if errors.Is(err, windows.ERROR_SERVICE_DISABLED) {
		// The disabled check precedes the running check: a running service
		// marked for deletion reports ERROR_SERVICE_DISABLED instead of
		// ERROR_SERVICE_ALREADY_RUNNING. The device is nonetheless up.
		var status windows.SERVICE_STATUS
		queryErr := windows.QueryServiceStatus(service, &status)
		if queryErr == nil && status.CurrentState == windows.SERVICE_RUNNING {
			return nil
		}
	}
	return E.Cause(err, "windivert: start service")
}

func openOrCreateService(manager windows.Handle, serviceNameW, sysPathW *uint16) (windows.Handle, error) {
	service, err := windows.OpenService(manager, serviceNameW, windows.SERVICE_ALL_ACCESS)
	if err == nil {
		return service, nil
	}
	service, err = windows.CreateService(
		manager,
		serviceNameW,
		serviceNameW,
		windows.SERVICE_ALL_ACCESS,
		windows.SERVICE_KERNEL_DRIVER,
		windows.SERVICE_DEMAND_START,
		windows.SERVICE_ERROR_NORMAL,
		sysPathW,
		nil, nil, nil, nil, nil,
	)
	if err == nil {
		return service, nil
	}
	if errors.Is(err, windows.ERROR_SERVICE_EXISTS) {
		service, err = windows.OpenService(manager, serviceNameW, windows.SERVICE_ALL_ACCESS)
		if err == nil {
			return service, nil
		}
	}
	return 0, wrapDriverInstallError(err)
}

func wrapDriverInstallError(err error) error {
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return E.Cause(err, "windivert: installing the kernel driver requires Administrator privileges")
	}
	return E.Cause(err, "windivert: create service")
}

// The cache directory is user-writable, so the .sys found there is
// untrusted: anything (e.g. a validly signed but vulnerable foreign driver)
// could have been planted before we run elevated. The bytes are therefore
// verified against the embedded asset through the returned handle, whose
// share mode denies write, delete, and rename until the caller closes it —
// the kernel maps exactly what was verified. MmLoadSystemImage opens the
// image with read/execute desired access, which the FILE_SHARE_READ grant
// admits, so holding the handle across StartService does not fail the load.
func extractVerified() (string, *os.File, error) {
	if len(sysBytes) == 0 {
		return "", nil, E.New("windivert: unsupported architecture ", runtime.GOARCH)
	}

	base, err := os.UserCacheDir()
	if err != nil {
		return "", nil, E.Cause(err, "windivert: locate user cache dir")
	}
	dir := filepath.Join(base, "sing-box", "windivert", "v"+AssetVersion)
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return "", nil, E.Cause(err, "windivert: mkdir ", dir)
	}
	target := filepath.Join(dir, driverSysName())

	for attempt := 0; ; attempt++ {
		sysFile, err := openDriverFile(target)
		if err != nil {
			if !os.IsNotExist(err) {
				return "", nil, E.Cause(err, "windivert: open ", target)
			}
			err = writeDriverFile(target)
			if err != nil {
				return "", nil, err
			}
			sysFile, err = openDriverFile(target)
			if err != nil {
				return "", nil, E.Cause(err, "windivert: open ", target)
			}
		}
		content, err := io.ReadAll(sysFile)
		if err != nil {
			sysFile.Close()
			return "", nil, E.Cause(err, "windivert: read ", target)
		}
		if bytes.Equal(content, sysBytes) {
			return target, sysFile, nil
		}
		sysFile.Close()
		if attempt > 0 {
			return "", nil, E.New("windivert: driver file ", target, " is being concurrently modified")
		}
		err = writeDriverFile(target)
		if err != nil {
			return "", nil, err
		}
	}
}

func openDriverFile(path string) (*os.File, error) {
	pathW, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		pathW,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

func writeDriverFile(target string) error {
	tmp := target + ".tmp-" + strconv.Itoa(os.Getpid())
	err := os.WriteFile(tmp, sysBytes, 0o644)
	if err != nil {
		return E.Cause(err, "windivert: write ", filepath.Base(target))
	}
	err = os.Rename(tmp, target)
	if err != nil {
		os.Remove(tmp)
		return E.Cause(err, "windivert: rename ", filepath.Base(target))
	}
	return nil
}
