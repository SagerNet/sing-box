//go:build windows

package vboxusb

import (
	"encoding/binary"
	"time"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

// DeviceRestart drives the cfgmgr32 restart-device sequence VBoxUSBMon
// depends on: the monitor only rewrites a device's IDs (and thereby
// hands it to VBoxUSB.sys, or back to its function driver) while PnP
// re-enumerates the device, which never happens for a device that is
// already sitting configured on the bus. Begin removes the devnode
// without restarting it; the caller mutates filters in between; Finish
// cycles the hub port (a software unplug/replug, so drivers see a
// clean device) and re-enables the devnode.
//
// Mirrors usbipd-win's RestartingDevice.
type DeviceRestart struct {
	devInst uint32
	hubPath string
	port    uint32
}

const (
	cmLocateDevNodeNormal   = 0x00000000
	cmRemoveUINotOK         = 0x00000001
	cmRemoveNoRestart       = 0x00000002
	cmSetupDevNodeReady     = 0x00000000
	maxDeviceIDLength       = 200
	ioctlUSBHubCyclePort    = 0x0022_0444 // CTL_CODE(FILE_DEVICE_USB, USB_HUB_CYCLE_PORT=273, METHOD_BUFFERED, FILE_ANY_ACCESS)
	deviceRestartSettleTime = 100 * time.Millisecond
)

// BeginDeviceRestart resolves the devnode and its parent hub, then
// removes the device subtree without restart. Call Finish to bring the
// device back; the pair must not be left half-open.
func BeginDeviceRestart(instanceID string, port uint32) (*DeviceRestart, error) {
	instanceW, err := windows.UTF16PtrFromString(instanceID)
	if err != nil {
		return nil, E.Cause(err, "vboxusb: utf16 instance id")
	}
	var devInst uint32
	ret, _, _ := procCMLocateDevNodeW.Call(
		uintptr(unsafe.Pointer(&devInst)),
		uintptr(unsafe.Pointer(instanceW)),
		cmLocateDevNodeNormal,
	)
	if windows.CONFIGRET(ret) != windows.CR_SUCCESS {
		return nil, E.New("vboxusb: CM_Locate_DevNode(", instanceID, ") CR=", ret)
	}
	hubPath := parentHubInterfacePath(devInst)
	var vetoType uint32
	var vetoName [260]uint16
	ret, _, _ = procCMQueryAndRemoveSubTreeW.Call(
		uintptr(devInst),
		uintptr(unsafe.Pointer(&vetoType)),
		uintptr(unsafe.Pointer(&vetoName[0])),
		uintptr(len(vetoName)),
		cmRemoveNoRestart|cmRemoveUINotOK,
	)
	if windows.CONFIGRET(ret) != windows.CR_SUCCESS {
		return nil, E.New("vboxusb: CM_Query_And_Remove_SubTree(", instanceID, ") CR=", ret,
			" veto=", vetoType, " by ", windows.UTF16ToString(vetoName[:]))
	}
	return &DeviceRestart{devInst: devInst, hubPath: hubPath, port: port}, nil
}

// Finish re-enumerates the removed device. Errors are swallowed by
// design (mirrors upstream): the device may have been physically
// unplugged meanwhile, or re-enumerated by someone else already.
func (r *DeviceRestart) Finish() {
	// Give the just-switched driver stack time to settle; upstream
	// found flash drives fail to re-initialize without this.
	time.Sleep(deviceRestartSettleTime)
	cycleHubPort(r.hubPath, r.port)
	_, _, _ = procCMSetupDevNode.Call(uintptr(r.devInst), cmSetupDevNodeReady)
}

// parentHubInterfacePath resolves the USB hub interface path of the
// device's parent before removal (afterwards the parent link may be
// unreliable). Empty on failure; Finish then skips the port cycle.
func parentHubInterfacePath(devInst uint32) string {
	var parent uint32
	ret, _, _ := procCMGetParent.Call(
		uintptr(unsafe.Pointer(&parent)),
		uintptr(devInst),
		0,
	)
	if windows.CONFIGRET(ret) != windows.CR_SUCCESS {
		return ""
	}
	var parentID [maxDeviceIDLength + 1]uint16
	ret, _, _ = procCMGetDeviceIDW.Call(
		uintptr(parent),
		uintptr(unsafe.Pointer(&parentID[0])),
		uintptr(len(parentID)),
		0,
	)
	if windows.CONFIGRET(ret) != windows.CR_SUCCESS {
		return ""
	}
	paths, err := windows.CM_Get_Device_Interface_List(
		windows.UTF16ToString(parentID[:]),
		&usbHubInterfaceGUID,
		windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT,
	)
	if err != nil {
		return ""
	}
	for _, p := range paths {
		if p != "" {
			return p
		}
	}
	return ""
}

// cycleHubPort issues IOCTL_USB_HUB_CYCLE_PORT — a software
// unplug/replug of the given port. Best effort.
func cycleHubPort(hubPath string, port uint32) {
	if hubPath == "" || port == 0 {
		return
	}
	hub := openHub(hubPath)
	if hub == windows.InvalidHandle {
		return
	}
	defer windows.CloseHandle(hub)
	// USB_CYCLE_PORT_PARAMS: ConnectionIndex (in) + StatusReturned (out).
	var params [8]byte
	binary.LittleEndian.PutUint32(params[0:4], port)
	var returned uint32
	_ = windows.DeviceIoControl(
		hub,
		ioctlUSBHubCyclePort,
		&params[0], uint32(len(params)),
		&params[0], uint32(len(params)),
		&returned, nil,
	)
}

var (
	modCfgMgr32                  = windows.NewLazySystemDLL("cfgmgr32.dll")
	procCMLocateDevNodeW         = modCfgMgr32.NewProc("CM_Locate_DevNodeW")
	procCMGetParent              = modCfgMgr32.NewProc("CM_Get_Parent")
	procCMGetDeviceIDW           = modCfgMgr32.NewProc("CM_Get_Device_IDW")
	procCMQueryAndRemoveSubTreeW = modCfgMgr32.NewProc("CM_Query_And_Remove_SubTreeW")
	procCMSetupDevNode           = modCfgMgr32.NewProc("CM_Setup_DevNode")
)
