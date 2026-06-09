//go:build windows

package usbipvhci

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

// classGUIDDevClassUSB is GUID_DEVCLASS_USB (devguid.h) — the setup class
// the VHCI devnode is created under, matching usbip2_ude.inf Class=USB.
var classGUIDDevClassUSB = windows.GUID{
	Data1: 0x36fc9e60,
	Data2: 0xc465,
	Data3: 0x11cf,
	Data4: [8]byte{0x80, 0x56, 0x44, 0x45, 0x53, 0x54, 0x00, 0x00},
}

// EnsureDriver extracts the bundled usbip-win2 UDE driver, registers the
// upper-filter package in the driver store, and creates the
// root-enumerated VHCI devnode when it is not already present. Requires
// Administrator. Idempotent; a named global mutex serializes concurrent
// installs, and an already-present interface is the fast-path no-op.
func EnsureDriver() error {
	driverOnce.Do(func() {
		driverErr = installDriver()
	})
	return driverErr
}

var (
	driverOnce sync.Once
	driverErr  error
)

func installDriver() error {
	controller, err := Open()
	if err == nil {
		_ = controller.Close()
		return nil
	}

	dir, err := ensureExtracted()
	if err != nil {
		return err
	}

	mutexName, _ := windows.UTF16PtrFromString(`Global\SingBoxUSBIPVHCIInstallMutex`)
	mutex, err := windows.CreateMutex(nil, false, mutexName)
	if err != nil {
		return E.Cause(err, "usbipvhci: create install mutex")
	}
	defer windows.CloseHandle(mutex)
	_, err = windows.WaitForSingleObject(mutex, windows.INFINITE)
	if err != nil {
		return E.Cause(err, "usbipvhci: wait install mutex")
	}
	defer windows.ReleaseMutex(mutex)

	// Another process may have completed the install while we waited.
	controller, err = Open()
	if err == nil {
		_ = controller.Close()
		return nil
	}

	// The upper filter is an extension INF: copying it into the driver
	// store is enough; PnP applies it once the VHCI root hub appears.
	err = addToDriverStore(filepath.Join(dir, "usbip2_filter.inf"))
	if err != nil {
		return E.Cause(err, "usbipvhci: register upper-filter driver")
	}
	err = createDevnodeAndInstall(filepath.Join(dir, "usbip2_ude.inf"))
	if err != nil {
		return E.Cause(err, "usbipvhci: create VHCI devnode")
	}
	return waitForInterface(20 * time.Second)
}

// addToDriverStore imports an INF (and its catalog-verified payload) into
// the Windows driver store. SetupCopyOEMInfW rejects unsigned or tampered
// packages; ERROR_FILE_EXISTS means the package is already present.
func addToDriverStore(infPath string) error {
	infW, err := windows.UTF16PtrFromString(infPath)
	if err != nil {
		return E.Cause(err, "usbipvhci: utf16 inf path")
	}
	dirW, err := windows.UTF16PtrFromString(filepath.Dir(infPath))
	if err != nil {
		return E.Cause(err, "usbipvhci: utf16 inf dir")
	}
	const spostPath = 1
	ret, _, callErr := procSetupCopyOEMInfW.Call(
		uintptr(unsafe.Pointer(infW)),
		uintptr(unsafe.Pointer(dirW)),
		uintptr(spostPath),
		0, 0, 0, 0, 0,
	)
	if ret == 0 && !errors.Is(callErr, windows.ERROR_FILE_EXISTS) {
		if errors.Is(callErr, windows.ERROR_ACCESS_DENIED) {
			return E.Cause(callErr, "SetupCopyOEMInfW (Administrator required)")
		}
		return E.Cause(callErr, "SetupCopyOEMInfW")
	}
	return nil
}

// createDevnodeAndInstall creates the root-enumerated VHCI devnode and
// installs usbip2_ude.inf onto it — the native equivalent of usbip-win2's
// "devnode install". infPath must be absolute. If a VHCI devnode already
// exists (e.g. the driver was removed but the root node lingers), the
// driver is reinstalled onto it instead of creating a duplicate.
func createDevnodeAndInstall(infPath string) error {
	err := updateDriverForPlugAndPlayDevices(udeHardwareID, infPath)
	if err == nil {
		return nil
	}

	devInfoSet, err := windows.SetupDiCreateDeviceInfoListEx(&classGUIDDevClassUSB, 0, "")
	if err != nil {
		return E.Cause(err, "SetupDiCreateDeviceInfoListEx")
	}
	defer devInfoSet.Close()

	devInfoData, err := windows.SetupDiCreateDeviceInfo(devInfoSet, "USB", &classGUIDDevClassUSB, "", 0, windows.DICD_GENERATE_ID)
	if err != nil {
		return E.Cause(err, "SetupDiCreateDeviceInfo")
	}

	err = windows.SetupDiSetDeviceRegistryProperty(devInfoSet, devInfoData, windows.SPDRP_HARDWAREID, multiSzUTF16(udeHardwareID))
	if err != nil {
		return E.Cause(err, "SetupDiSetDeviceRegistryProperty")
	}
	err = windows.SetupDiCallClassInstaller(windows.DIF_REGISTERDEVICE, devInfoSet, devInfoData)
	if err != nil {
		return E.Cause(err, "SetupDiCallClassInstaller(DIF_REGISTERDEVICE)")
	}
	return updateDriverForPlugAndPlayDevices(udeHardwareID, infPath)
}

// updateDriverForPlugAndPlayDevices binds the INF driver to every present
// device matching hardwareID (the devnode just created). INSTALLFLAG_FORCE
// installs even when the bundled driver is not strictly newer.
func updateDriverForPlugAndPlayDevices(hardwareID, infPath string) error {
	hardwareIDW, err := windows.UTF16PtrFromString(hardwareID)
	if err != nil {
		return E.Cause(err, "usbipvhci: utf16 hardware id")
	}
	infW, err := windows.UTF16PtrFromString(infPath)
	if err != nil {
		return E.Cause(err, "usbipvhci: utf16 inf path")
	}
	const installFlagForce = 0x00000001
	var rebootRequired int32
	ret, _, callErr := procUpdateDriverForPlugAndPlayDevicesW.Call(
		0,
		uintptr(unsafe.Pointer(hardwareIDW)),
		uintptr(unsafe.Pointer(infW)),
		uintptr(installFlagForce),
		uintptr(unsafe.Pointer(&rebootRequired)),
	)
	if ret == 0 {
		return E.Cause(callErr, "UpdateDriverForPlugAndPlayDevices")
	}
	return nil
}

func waitForInterface(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		controller, err := Open()
		if err == nil {
			_ = controller.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return E.Cause(err, "usbipvhci: VHCI interface did not appear after install (the devnode may have failed to start; check Device Manager)")
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// multiSzUTF16 encodes s as a REG_MULTI_SZ (UTF-16LE, double-NUL terminated).
func multiSzUTF16(s string) []byte {
	u16, err := windows.UTF16FromString(s)
	if err != nil {
		return nil
	}
	u16 = append(u16, 0) // second NUL ends the list
	buf := make([]byte, len(u16)*2)
	for i, v := range u16 {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	return buf
}

var (
	extractOnce sync.Once
	extractErr  error
	extractDir  string
)

func ensureExtracted() (string, error) {
	extractOnce.Do(func() {
		extractDir, extractErr = extractImpl()
	})
	return extractDir, extractErr
}

func extractImpl() (string, error) {
	files := assetFiles()
	if len(files) == 0 {
		return "", E.New("usbipvhci: no bundled driver for ", runtime.GOARCH)
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", E.Cause(err, "usbipvhci: locate user cache dir")
	}
	dir := filepath.Join(base, "sing-box", "usbipvhci", "v"+assetVersion())
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return "", E.Cause(err, "usbipvhci: mkdir ", dir)
	}
	for _, asset := range files {
		err = ensureAsset(dir, asset)
		if err != nil {
			return "", err
		}
	}
	return dir, nil
}

// ensureAsset writes one asset atomically: concurrent sing-box processes
// race on os.Rename (atomic on NTFS) and losers discard their temp copy.
func ensureAsset(dir string, asset assetFile) error {
	target := filepath.Join(dir, asset.name)
	_, err := os.Stat(target)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return E.Cause(err, "usbipvhci: stat ", asset.name)
	}
	tmp := target + ".tmp-" + strconv.Itoa(os.Getpid())
	err = os.WriteFile(tmp, asset.data, 0o644)
	if err != nil {
		return E.Cause(err, "usbipvhci: write ", asset.name)
	}
	err = os.Rename(tmp, target)
	if err != nil {
		os.Remove(tmp)
		_, statErr := os.Stat(target)
		if statErr == nil {
			return nil
		}
		return E.Cause(err, "usbipvhci: rename ", asset.name)
	}
	return nil
}

var (
	modSetupAPI          = windows.NewLazyDLL("setupapi.dll")
	procSetupCopyOEMInfW = modSetupAPI.NewProc("SetupCopyOEMInfW")

	modNewDev                              = windows.NewLazyDLL("newdev.dll")
	procUpdateDriverForPlugAndPlayDevicesW = modNewDev.NewProc("UpdateDriverForPlugAndPlayDevicesW")
)
