//go:build windows

package vboxusb

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

// EnsureDrivers extracts the bundled VBoxUSB + VBoxUSBMon binaries and
// makes both drivers available to PnP:
//
//  1. VBoxUSBMon is registered as a SERVICE_KERNEL_DRIVER demand-start
//     service. The actual handle to \\.\VBoxUSBMon is opened separately
//     via OpenMonitor.
//  2. VBoxUSB.inf is copied into the driver store via SetupCopyOEMInfW
//     so PnP knows the driver exists; actual per-device binding is
//     orchestrated later via VBoxUSBMon's ADD_FILTER mechanism.
//
// Requires Administrator (SeLoadDriverPrivilege). Safe to call from
// multiple processes concurrently; a named global mutex serializes
// the install.
func EnsureDrivers() error {
	driverOnce.Do(func() {
		driverErr = installDrivers()
	})
	return driverErr
}

var (
	driverOnce sync.Once
	driverErr  error
)

func installDrivers() error {
	dir, err := ensureExtracted()
	if err != nil {
		return err
	}

	mutexName, _ := windows.UTF16PtrFromString("Global\\SingBoxVBoxUSBInstallMutex")
	mutex, err := windows.CreateMutex(nil, false, mutexName)
	if err != nil {
		return E.Cause(err, "vboxusb: create install mutex")
	}
	defer windows.CloseHandle(mutex)
	_, err = windows.WaitForSingleObject(mutex, windows.INFINITE)
	if err != nil {
		return E.Cause(err, "vboxusb: wait install mutex")
	}
	defer windows.ReleaseMutex(mutex)

	err = installMonitorService(dir)
	if err != nil {
		return err
	}
	err = installVBoxUSBInf(dir)
	if err != nil {
		return err
	}
	return nil
}

func installMonitorService(dir string) error {
	sysPath := filepath.Join(dir, "VBoxUSBMon.sys")
	sysPathW, err := windows.UTF16PtrFromString(sysPath)
	if err != nil {
		return E.Cause(err, "vboxusb: utf16 monitor path")
	}

	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_ALL_ACCESS)
	if err != nil {
		return E.Cause(err, "vboxusb: open SCM")
	}
	defer windows.CloseServiceHandle(manager)

	serviceNameW, _ := windows.UTF16PtrFromString(MonitorServiceName)
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
				return wrapInstallError(err)
			}
		}
	}
	defer windows.CloseServiceHandle(service)

	err = windows.StartService(service, 0, nil)
	if err != nil && errors.Is(err, windows.ERROR_SERVICE_DISABLED) {
		// A prior process called DeleteService on a still-loaded
		// driver: SCM marks the record for deletion and flips
		// START_TYPE to DISABLED until the last handle closes.
		// Re-enable so we can start instead of waiting for a reboot.
		err = windows.ChangeServiceConfig(
			service,
			windows.SERVICE_NO_CHANGE,
			windows.SERVICE_DEMAND_START,
			windows.SERVICE_NO_CHANGE,
			nil, nil, nil, nil, nil, nil, nil,
		)
		if err != nil {
			return E.Cause(err, "vboxusb: re-enable disabled monitor service")
		}
		err = windows.StartService(service, 0, nil)
	}
	if err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return E.Cause(err, "vboxusb: start monitor service")
	}
	return nil
}

func wrapInstallError(err error) error {
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return E.Cause(err, "vboxusb: installing the kernel driver requires Administrator privileges")
	}
	return E.Cause(err, "vboxusb: create monitor service")
}

// installVBoxUSBInf copies VBoxUSB.inf into the Windows driver store
// via SetupCopyOEMInfW so PnP knows the driver is available. The
// actual per-device binding is triggered later by VBoxUSBMon's
// ADD_FILTER + a port cycle.
//
// SetupCopyOEMInfW is not exposed by golang.org/x/sys/windows, so we
// resolve it manually via the setupapi DLL.
func installVBoxUSBInf(dir string) error {
	infPath := filepath.Join(dir, "VBoxUSB.inf")
	infPathW, err := windows.UTF16PtrFromString(infPath)
	if err != nil {
		return E.Cause(err, "vboxusb: utf16 inf path")
	}
	dirW, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return E.Cause(err, "vboxusb: utf16 inf dir")
	}
	const (
		spostPath     = 1 // SPOST_PATH
		spCopyNoStyle = 0
	)
	ret, _, callErr := procSetupCopyOEMInfW.Call(
		uintptr(unsafe.Pointer(infPathW)),
		uintptr(unsafe.Pointer(dirW)),
		uintptr(spostPath),
		uintptr(spCopyNoStyle),
		0, 0, 0, 0,
	)
	if ret == 0 {
		if errors.Is(callErr, windows.ERROR_FILE_EXISTS) {
			// Already present in driver store; not an error.
			return nil
		}
		return E.Cause(callErr, "vboxusb: SetupCopyOEMInfW")
	}
	return nil
}

var (
	modSetupAPI          = windows.NewLazyDLL("setupapi.dll")
	procSetupCopyOEMInfW = modSetupAPI.NewProc("SetupCopyOEMInfW")
)

type assetFile struct {
	name string
	data []byte
}

var (
	extractOnce sync.Once
	extractErr  error
	extractDir  string
)

// The on-disk copy is protected by Authenticode signature enforcement
// at SCM StartService time; any tampering with the .sys is rejected
// by the kernel loader before we ever see it.
func ensureExtracted() (string, error) {
	extractOnce.Do(func() {
		extractDir, extractErr = extractImpl()
	})
	return extractDir, extractErr
}

func extractImpl() (string, error) {
	files := assetFiles()
	if len(files) == 0 {
		return "", E.New("vboxusb: unsupported architecture ", runtime.GOARCH)
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", E.Cause(err, "vboxusb: locate user cache dir")
	}
	dir := filepath.Join(base, "sing-box", "vboxusb", "v"+AssetVersion)
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return "", E.Cause(err, "vboxusb: mkdir ", dir)
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
		return E.Cause(err, "vboxusb: stat ", asset.name)
	}
	tmp := target + ".tmp-" + strconv.Itoa(os.Getpid())
	err = os.WriteFile(tmp, asset.data, 0o644)
	if err != nil {
		return E.Cause(err, "vboxusb: write ", asset.name)
	}
	err = os.Rename(tmp, target)
	if err != nil {
		os.Remove(tmp)
		_, statErr := os.Stat(target)
		if statErr == nil {
			return nil
		}
		return E.Cause(err, "vboxusb: rename ", asset.name)
	}
	return nil
}
