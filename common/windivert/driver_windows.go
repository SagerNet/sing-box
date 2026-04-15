//go:build windows

package windivert

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

const (
	driverServiceName = "WinDivert"
	driverDeviceName  = `\\.\WinDivert`
)

var (
	driverOnce sync.Once
	driverErr  error
	// driverDevName is ASCII-safe and must be available before ensureDriver
	// so Open can try CreateFile first and only install on FILE_NOT_FOUND.
	driverDevName, _ = windows.UTF16PtrFromString(driverDeviceName)
)

// Requires SeLoadDriverPrivilege (Administrator). Running the 386 build
// under WOW64 on a 64-bit kernel is rejected — use the amd64 build.
func ensureDriver() error {
	driverOnce.Do(func() {
		driverErr = installDriver()
	})
	return driverErr
}

func installDriver() error {
	if runtime.GOARCH == "386" {
		var isWow64 bool
		err := windows.IsWow64Process(windows.CurrentProcess(), &isWow64)
		if err == nil && isWow64 {
			return E.New("windivert: 386 build detected running under WOW64 on a 64-bit kernel; use the amd64 build")
		}
	}

	dir, err := ensureExtracted()
	if err != nil {
		return err
	}
	sysPath := filepath.Join(dir, driverSysName())
	sysPathW, err := windows.UTF16PtrFromString(sysPath)
	if err != nil {
		return E.Cause(err, "windivert: utf16 driver path")
	}

	// Serialize driver install across concurrent processes.
	mutexName, _ := windows.UTF16PtrFromString("WinDivertDriverInstallMutex")
	mutex, err := windows.CreateMutex(nil, false, mutexName)
	if err != nil {
		return E.Cause(err, "windivert: create install mutex")
	}
	defer windows.CloseHandle(mutex)
	_, err = windows.WaitForSingleObject(mutex, windows.INFINITE)
	if err != nil {
		return E.Cause(err, "windivert: wait install mutex")
	}
	defer windows.ReleaseMutex(mutex)

	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_ALL_ACCESS)
	if err != nil {
		return E.Cause(err, "windivert: open SCM")
	}
	defer windows.CloseServiceHandle(manager)

	serviceNameW, _ := windows.UTF16PtrFromString(driverServiceName)
	service, err := windows.OpenService(manager, serviceNameW, windows.SERVICE_ALL_ACCESS)
	if err != nil {
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
		if err != nil {
			if errors.Is(err, windows.ERROR_SERVICE_EXISTS) {
				service, err = windows.OpenService(manager, serviceNameW, windows.SERVICE_ALL_ACCESS)
			}
			if err != nil {
				return wrapDriverInstallError(err)
			}
		}
	}
	defer windows.CloseServiceHandle(service)

	err = windows.StartService(service, 0, nil)
	if err != nil && errors.Is(err, windows.ERROR_SERVICE_DISABLED) {
		// A prior process called DeleteService on a still-running kernel
		// driver: SCM marks the record for deletion and flips START_TYPE
		// to DISABLED until the last handle closes. Re-enable so we can
		// start it instead of waiting for a reboot.
		err = windows.ChangeServiceConfig(
			service,
			windows.SERVICE_NO_CHANGE,
			windows.SERVICE_DEMAND_START,
			windows.SERVICE_NO_CHANGE,
			nil, nil, nil, nil, nil, nil, nil,
		)
		if err != nil {
			return E.Cause(err, "windivert: re-enable disabled service")
		}
		err = windows.StartService(service, 0, nil)
	}
	if err == nil {
		// Mark for deletion so the driver unregisters when the last handle
		// closes or on next reboot. Matches the upstream DLL's behavior:
		// only the process that actually started the service takes on the
		// cleanup responsibility. If another process already started it,
		// we leave DeleteService to them.
		_ = windows.DeleteService(service)
	} else if !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return E.Cause(err, "windivert: start service")
	}
	return nil
}

func wrapDriverInstallError(err error) error {
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return E.Cause(err, "windivert: installing the kernel driver requires Administrator privileges")
	}
	return E.Cause(err, "windivert: create service")
}

type assetFile struct {
	name string
	data []byte
}

var (
	extractOnce sync.Once
	extractErr  error
	extractDir  string
)

// The on-disk copy is protected by Windows Authenticode signature
// enforcement, which rejects any tampered .sys at StartService time.
func ensureExtracted() (string, error) {
	extractOnce.Do(func() {
		extractDir, extractErr = extractImpl()
	})
	return extractDir, extractErr
}

func extractImpl() (string, error) {
	files := assetFiles()
	if len(files) == 0 {
		return "", E.New("windivert: unsupported architecture ", runtime.GOARCH)
	}

	base, err := os.UserCacheDir()
	if err != nil {
		return "", E.Cause(err, "windivert: locate user cache dir")
	}
	dir := filepath.Join(base, "sing-box", "windivert", "v"+AssetVersion)
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return "", E.Cause(err, "windivert: mkdir ", dir)
	}

	for _, asset := range files {
		err = ensureAsset(dir, asset)
		if err != nil {
			return "", err
		}
	}
	return dir, nil
}

// Concurrent sing-box processes race on os.Rename (atomic on NTFS);
// whichever wins creates the final file. Writers that lose the race
// silently discard their temp copy.
func ensureAsset(dir string, asset assetFile) error {
	target := filepath.Join(dir, asset.name)
	_, err := os.Stat(target)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return E.Cause(err, "windivert: stat ", asset.name)
	}
	tmp := target + ".tmp-" + strconv.Itoa(os.Getpid())
	err = os.WriteFile(tmp, asset.data, 0o644)
	if err != nil {
		return E.Cause(err, "windivert: write ", asset.name)
	}
	err = os.Rename(tmp, target)
	if err != nil {
		os.Remove(tmp)
		if _, statErr := os.Stat(target); statErr == nil {
			return nil
		}
		return E.Cause(err, "windivert: rename ", asset.name)
	}
	return nil
}
