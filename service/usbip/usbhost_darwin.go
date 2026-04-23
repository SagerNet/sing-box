//go:build darwin && cgo

package usbip

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -fblocks
#cgo LDFLAGS: -framework Foundation -framework IOKit -framework IOUSBHost

#include <stdlib.h>
#include "usbhost_darwin.h"
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

const (
	ciStatusSuccess         = 1
	ciStatusOffline         = 2
	ciStatusNotPermitted    = 3
	ciStatusBadArgument     = 4
	ciStatusTimeout         = 5
	ciStatusNoResources     = 6
	ciStatusEndpointStopped = 7
	ciStatusStallError      = 11
	ciStatusError           = 13

	ciMsgControllerPowerOn     = 0x10
	ciMsgControllerPowerOff    = 0x11
	ciMsgControllerStart       = 0x12
	ciMsgControllerPause       = 0x13
	ciMsgControllerFrameNumber = 0x14
	ciMsgPortPowerOn           = 0x18
	ciMsgPortPowerOff          = 0x19
	ciMsgPortResume            = 0x1a
	ciMsgPortSuspend           = 0x1b
	ciMsgPortReset             = 0x1c
	ciMsgPortDisable           = 0x1d
	ciMsgPortStatus            = 0x1e
	ciMsgDeviceCreate          = 0x20
	ciMsgDeviceDestroy         = 0x21
	ciMsgDeviceStart           = 0x22
	ciMsgDevicePause           = 0x23
	ciMsgDeviceUpdate          = 0x24
	ciMsgEndpointCreate        = 0x28
	ciMsgEndpointDestroy       = 0x29
	ciMsgEndpointPause         = 0x2b
	ciMsgEndpointUpdate        = 0x2c
	ciMsgEndpointReset         = 0x2d
	ciMsgEndpointSetNext       = 0x2e
	ciMsgSetupTransfer         = 0x38
	ciMsgNormalTransfer        = 0x39
	ciMsgStatusTransfer        = 0x3a
	ciMsgIsochronousTransfer   = 0x3b
)

type darwinCIMessage struct {
	control uint32
	data0   uint32
	data1   uint64
	buffer  unsafe.Pointer
}

type darwinCITransfer struct {
	ptr     unsafe.Pointer
	message darwinCIMessage
}

type darwinUSBHostController struct {
	handle *C.box_usbhost_controller_t
	ref    cgo.Handle
}

type darwinUSBHostDeviceSM struct {
	handle *C.box_usbhost_device_sm_t
}

type darwinUSBHostEndpointSM struct {
	handle *C.box_usbhost_endpoint_sm_t
}

type darwinUSBHostDeviceInfo struct {
	registryID uint64
	entry      DeviceEntry
	key        DeviceKey
}

type darwinUSBHostDevice struct {
	handle *C.box_usbhost_device_t
	info   darwinUSBHostDeviceInfo
}

func darwinCopyUSBHostDevices() ([]darwinUSBHostDeviceInfo, error) {
	var list C.box_usbhost_device_list_t
	var errorPtr *C.char
	if !bool(C.box_usbhost_copy_devices(&list, &errorPtr)) {
		return nil, darwinCError(errorPtr)
	}
	defer C.box_usbhost_device_list_free(&list)
	if list.count == 0 {
		return nil, nil
	}
	raw := unsafe.Slice(list.items, int(list.count))
	devices := make([]darwinUSBHostDeviceInfo, 0, len(raw))
	for i := range raw {
		devices = append(devices, darwinDeviceInfoFromC(&raw[i]))
	}
	return devices, nil
}

func darwinOpenUSBHostDevice(registryID uint64, capture bool) (*darwinUSBHostDevice, error) {
	var info C.box_usbhost_device_info_t
	var errorPtr *C.char
	handle := C.box_usbhost_device_open(C.uint64_t(registryID), C.bool(capture), &info, &errorPtr)
	if handle == nil {
		return nil, darwinCError(errorPtr)
	}
	return &darwinUSBHostDevice{
		handle: handle,
		info:   darwinDeviceInfoFromC(&info),
	}, nil
}

func darwinCreateUSBHostController(controller *darwinVirtualController, portCount uint8, speed uint32) (*darwinUSBHostController, error) {
	ref := cgo.NewHandle(controller)
	var errorPtr *C.char
	handle := C.box_usbhost_controller_create(C.uintptr_t(ref), C.uint8_t(portCount), C.uint32_t(speed), &errorPtr)
	if handle == nil {
		ref.Delete()
		return nil, darwinCError(errorPtr)
	}
	return &darwinUSBHostController{handle: handle, ref: ref}, nil
}

func (c *darwinUSBHostController) Close() {
	if c == nil || c.handle == nil {
		return
	}
	C.box_usbhost_controller_destroy(c.handle)
	c.handle = nil
	c.ref.Delete()
}

func (c *darwinUSBHostController) respond(message darwinCIMessage, status int) error {
	cMessage := message.c()
	var errorPtr *C.char
	if !bool(C.box_usbhost_controller_respond(c.handle, &cMessage, C.int(status), &errorPtr)) {
		return darwinCError(errorPtr)
	}
	return nil
}

func (c *darwinUSBHostController) respondFrame(message darwinCIMessage, status int, frame uint64, timestamp uint64) error {
	cMessage := message.c()
	var errorPtr *C.char
	if !bool(C.box_usbhost_controller_respond_frame(c.handle, &cMessage, C.int(status), C.uint64_t(frame), C.uint64_t(timestamp), &errorPtr)) {
		return darwinCError(errorPtr)
	}
	return nil
}

func (c *darwinUSBHostController) respondPort(message darwinCIMessage, status int, powered bool, connected bool, speed uint32) error {
	cMessage := message.c()
	var errorPtr *C.char
	if !bool(C.box_usbhost_port_respond(c.handle, &cMessage, C.int(status), C.bool(powered), C.bool(connected), C.uint32_t(speed), &errorPtr)) {
		return darwinCError(errorPtr)
	}
	return nil
}

func (c *darwinUSBHostController) createDeviceSM(message darwinCIMessage) (*darwinUSBHostDeviceSM, error) {
	cMessage := message.c()
	var errorPtr *C.char
	handle := C.box_usbhost_device_sm_create(c.handle, &cMessage, &errorPtr)
	if handle == nil {
		return nil, darwinCError(errorPtr)
	}
	return &darwinUSBHostDeviceSM{handle: handle}, nil
}

func (s *darwinUSBHostDeviceSM) Close() {
	if s == nil || s.handle == nil {
		return
	}
	C.box_usbhost_device_sm_destroy(s.handle)
	s.handle = nil
}

func (s *darwinUSBHostDeviceSM) respond(message darwinCIMessage, status int) error {
	cMessage := message.c()
	var errorPtr *C.char
	if !bool(C.box_usbhost_device_sm_respond(s.handle, &cMessage, C.int(status), &errorPtr)) {
		return darwinCError(errorPtr)
	}
	return nil
}

func (s *darwinUSBHostDeviceSM) respondCreate(message darwinCIMessage, status int, deviceAddress uint8) error {
	cMessage := message.c()
	var errorPtr *C.char
	if !bool(C.box_usbhost_device_sm_respond_create(s.handle, &cMessage, C.int(status), C.uint8_t(deviceAddress), &errorPtr)) {
		return darwinCError(errorPtr)
	}
	return nil
}

func (c *darwinUSBHostController) createEndpointSM(message darwinCIMessage) (*darwinUSBHostEndpointSM, error) {
	cMessage := message.c()
	var errorPtr *C.char
	handle := C.box_usbhost_endpoint_sm_create(c.handle, &cMessage, &errorPtr)
	if handle == nil {
		return nil, darwinCError(errorPtr)
	}
	return &darwinUSBHostEndpointSM{handle: handle}, nil
}

func (s *darwinUSBHostEndpointSM) Close() {
	if s == nil || s.handle == nil {
		return
	}
	C.box_usbhost_endpoint_sm_destroy(s.handle)
	s.handle = nil
}

func (s *darwinUSBHostEndpointSM) respond(message darwinCIMessage, status int) error {
	cMessage := message.c()
	var errorPtr *C.char
	if !bool(C.box_usbhost_endpoint_sm_respond(s.handle, &cMessage, C.int(status), &errorPtr)) {
		return darwinCError(errorPtr)
	}
	return nil
}

func (s *darwinUSBHostEndpointSM) processDoorbell(doorbell uint32) error {
	var errorPtr *C.char
	if !bool(C.box_usbhost_endpoint_sm_process_doorbell(s.handle, C.uint32_t(doorbell), &errorPtr)) {
		return darwinCError(errorPtr)
	}
	return nil
}

func (s *darwinUSBHostEndpointSM) currentTransfer() darwinCITransfer {
	ptr := C.box_usbhost_endpoint_sm_current_transfer(s.handle)
	if ptr == nil {
		return darwinCITransfer{}
	}
	return darwinCITransfer{
		ptr: unsafe.Pointer(ptr),
		message: darwinCIMessage{
			control: uint32(ptr.control),
			data0:   uint32(ptr.data0),
			data1:   uint64(ptr.data1),
			buffer:  C.box_usbhost_ci_normal_buffer(ptr),
		},
	}
}

func (s *darwinUSBHostEndpointSM) complete(transfer darwinCITransfer, status int, length int) error {
	var errorPtr *C.char
	if !bool(C.box_usbhost_endpoint_sm_complete(s.handle, (*C.IOUSBHostCIMessage)(transfer.ptr), C.int(status), C.size_t(length), &errorPtr)) {
		return darwinCError(errorPtr)
	}
	return nil
}

func (m darwinCIMessage) c() C.IOUSBHostCIMessage {
	return C.IOUSBHostCIMessage{
		control: C.uint32_t(m.control),
		data0:   C.uint32_t(m.data0),
		data1:   C.uint64_t(m.data1),
	}
}

func (m darwinCIMessage) messageType() uint8 {
	return uint8(m.control & 0x3f)
}

func (m darwinCIMessage) valid() bool {
	return m.control&(1<<15) != 0
}

func (m darwinCIMessage) noResponse() bool {
	return m.control&(1<<14) != 0
}

func (m darwinCIMessage) deviceAddress() uint8 {
	return uint8(m.data0 & 0xff)
}

func (m darwinCIMessage) endpointAddress() uint8 {
	return uint8((m.data0 >> 8) & 0xff)
}

func (m darwinCIMessage) rootPort() uint8 {
	return uint8(m.data0 & 0x0f)
}

func (m darwinCIMessage) setup() [8]byte {
	var setup [8]byte
	value := m.data1
	for i := range setup {
		setup[i] = byte(value >> (8 * i))
	}
	return setup
}

func (m darwinCIMessage) normalLength() uint32 {
	return m.data0 & ((1 << 28) - 1)
}

func (m darwinCIMessage) bufferPointer() unsafe.Pointer {
	return m.buffer
}

func darwinCIFrameTimestamp() uint64 {
	return uint64(C.box_usbhost_now())
}

//export box_usbip_darwin_controller_command
func box_usbip_darwin_controller_command(ref C.uintptr_t, message C.IOUSBHostCIMessage) {
	handle := cgo.Handle(ref)
	controller, ok := handle.Value().(*darwinVirtualController)
	if !ok {
		return
	}
	controller.enqueueCommand(darwinCIMessage{
		control: uint32(message.control),
		data0:   uint32(message.data0),
		data1:   uint64(message.data1),
	})
}

//export box_usbip_darwin_controller_doorbell
func box_usbip_darwin_controller_doorbell(ref C.uintptr_t, doorbell C.uint32_t) {
	handle := cgo.Handle(ref)
	controller, ok := handle.Value().(*darwinVirtualController)
	if !ok {
		return
	}
	controller.enqueueDoorbell(uint32(doorbell))
}

func (d *darwinUSBHostDevice) Close() {
	if d == nil || d.handle == nil {
		return
	}
	C.box_usbhost_device_close(d.handle)
	d.handle = nil
}

func (d *darwinUSBHostDevice) control(setup [8]byte, buffer []byte) (int32, int32, []byte, error) {
	if d == nil || d.handle == nil {
		return -int32(unix.ENODEV), 0, nil, E.New("IOUSBHostDevice control: closed")
	}
	var actual C.size_t
	var status C.int32_t
	var errorPtr *C.char
	var dataPtr *C.uint8_t
	if len(buffer) > 0 {
		dataPtr = (*C.uint8_t)(unsafe.Pointer(&buffer[0]))
	}
	setupPtr := (*C.uint8_t)(unsafe.Pointer(&setup[0]))
	if !bool(C.box_usbhost_device_control(d.handle, setupPtr, dataPtr, C.size_t(len(buffer)), &actual, &status, &errorPtr)) {
		return -int32(unix.EIO), 0, nil, darwinCError(errorPtr)
	}
	return darwinIOReturnToUSBIPStatus(int32(status)), int32(actual), buffer, nil
}

func (d *darwinUSBHostDevice) io(endpoint uint8, buffer []byte) (int32, int32, []byte, error) {
	if d == nil || d.handle == nil {
		return -int32(unix.ENODEV), 0, nil, E.New("IOUSBHostPipe IO: closed")
	}
	var actual C.size_t
	var status C.int32_t
	var errorPtr *C.char
	var dataPtr *C.uint8_t
	if len(buffer) > 0 {
		dataPtr = (*C.uint8_t)(unsafe.Pointer(&buffer[0]))
	}
	if !bool(C.box_usbhost_device_io(d.handle, C.uint8_t(endpoint), dataPtr, C.size_t(len(buffer)), &actual, &status, &errorPtr)) {
		return -int32(unix.EIO), 0, nil, darwinCError(errorPtr)
	}
	return darwinIOReturnToUSBIPStatus(int32(status)), int32(actual), buffer, nil
}

func (d *darwinUSBHostDevice) iso(endpoint uint8, buffer []byte, startFrame int32, packets []IsoPacketDescriptor) (int32, int32, []byte, []IsoPacketDescriptor, error) {
	if d == nil || d.handle == nil {
		return -int32(unix.ENODEV), 0, nil, nil, E.New("IOUSBHostPipe isochronous IO: closed")
	}
	var actual C.size_t
	var status C.int32_t
	var errorPtr *C.char
	var dataPtr *C.uint8_t
	if len(buffer) > 0 {
		dataPtr = (*C.uint8_t)(unsafe.Pointer(&buffer[0]))
	}
	cPackets := make([]C.box_usbhost_iso_packet_t, len(packets))
	for i := range packets {
		cPackets[i].offset = C.int32_t(packets[i].Offset)
		cPackets[i].length = C.int32_t(packets[i].Length)
		cPackets[i].actual_length = C.int32_t(packets[i].ActualLength)
		cPackets[i].status = C.int32_t(packets[i].Status)
	}
	var packetsPtr *C.box_usbhost_iso_packet_t
	if len(cPackets) > 0 {
		packetsPtr = (*C.box_usbhost_iso_packet_t)(unsafe.Pointer(&cPackets[0]))
	}
	if !bool(C.box_usbhost_device_iso(d.handle, C.uint8_t(endpoint), dataPtr, C.size_t(len(buffer)), C.int32_t(startFrame), packetsPtr, C.size_t(len(cPackets)), &actual, &status, &errorPtr)) {
		return -int32(unix.EIO), 0, nil, nil, darwinCError(errorPtr)
	}
	for i := range packets {
		packets[i].ActualLength = int32(cPackets[i].actual_length)
		packets[i].Status = darwinIOReturnToUSBIPStatus(int32(cPackets[i].status))
	}
	return darwinIOReturnToUSBIPStatus(int32(status)), int32(actual), buffer, packets, nil
}

func (d *darwinUSBHostDevice) abortEndpoint(endpoint uint8) error {
	if d == nil || d.handle == nil {
		return nil
	}
	var errorPtr *C.char
	if !bool(C.box_usbhost_device_abort_endpoint(d.handle, C.uint8_t(endpoint), &errorPtr)) {
		return darwinCError(errorPtr)
	}
	return nil
}

func darwinDeviceInfoFromC(info *C.box_usbhost_device_info_t) darwinUSBHostDeviceInfo {
	busid := cCharArrayString(unsafe.Pointer(&info.busid[0]))
	serial := cCharArrayString(unsafe.Pointer(&info.serial[0]))
	path := cCharArrayString(unsafe.Pointer(&info.path[0]))
	entry := DeviceEntry{
		Info: DeviceInfoTruncated{
			BusNum:              uint32(info.busnum),
			DevNum:              uint32(info.devnum),
			Speed:               uint32(info.speed),
			IDVendor:            uint16(info.vendor_id),
			IDProduct:           uint16(info.product_id),
			BCDDevice:           uint16(info.bcd_device),
			BDeviceClass:        uint8(info.class_id),
			BDeviceSubClass:     uint8(info.subclass_id),
			BDeviceProtocol:     uint8(info.protocol_id),
			BConfigurationValue: uint8(info.configuration_value),
			BNumConfigurations:  uint8(info.num_configurations),
			BNumInterfaces:      uint8(info.interface_count),
		},
	}
	copy(entry.Info.BusID[:], busid)
	encodePathField(&entry.Info.Path, path, serial)
	interfaceCount := int(info.interface_count)
	if interfaceCount > C.BOX_USBHOST_MAX_INTERFACES {
		interfaceCount = C.BOX_USBHOST_MAX_INTERFACES
	}
	if interfaceCount > 0 {
		rawInterfaces := unsafe.Slice(&info.interfaces[0], interfaceCount)
		entry.Interfaces = make([]DeviceInterface, interfaceCount)
		for i := range rawInterfaces {
			entry.Interfaces[i] = DeviceInterface{
				BInterfaceClass:    uint8(rawInterfaces[i].class_id),
				BInterfaceSubClass: uint8(rawInterfaces[i].subclass_id),
				BInterfaceProtocol: uint8(rawInterfaces[i].protocol_id),
			}
		}
	}
	return darwinUSBHostDeviceInfo{
		registryID: uint64(info.registry_id),
		entry:      entry,
		key: DeviceKey{
			BusID:     busid,
			VendorID:  uint16(info.vendor_id),
			ProductID: uint16(info.product_id),
			Serial:    serial,
		},
	}
}

func cCharArrayString(ptr unsafe.Pointer) string {
	if ptr == nil {
		return ""
	}
	return C.GoString((*C.char)(ptr))
}

func darwinCError(errorPtr *C.char) error {
	if errorPtr == nil {
		return E.New("IOUSBHost: unknown error")
	}
	defer C.box_usbhost_free_error(errorPtr)
	return E.New(C.GoString(errorPtr))
}

func darwinIOReturnToUSBIPStatus(status int32) int32 {
	if status == 0 {
		return 0
	}
	switch status {
	case int32(C.kIOReturnAborted), int32(C.kIOReturnNotResponding):
		return -int32(unix.ECONNRESET)
	case int32(C.kIOReturnNoDevice), int32(C.kIOReturnOffline):
		return -int32(unix.ENODEV)
	case int32(C.kIOReturnNotPermitted):
		return -int32(unix.EPERM)
	case int32(C.kIOReturnTimeout):
		return -int32(unix.ETIMEDOUT)
	case int32(C.kIOReturnNoResources), int32(C.kIOReturnNoMemory):
		return -int32(unix.ENOMEM)
	case int32(C.kIOReturnBadArgument):
		return -int32(unix.EINVAL)
	default:
		return -int32(unix.EIO)
	}
}

func usbipStatusToDarwinCIStatus(status int32) C.int {
	if status == 0 {
		return C.IOUSBHostCIMessageStatusSuccess
	}
	switch -status {
	case int32(unix.ETIMEDOUT):
		return C.IOUSBHostCIMessageStatusTimeout
	case int32(unix.ENOMEM):
		return C.IOUSBHostCIMessageStatusNoResources
	case int32(unix.EINVAL):
		return C.IOUSBHostCIMessageStatusBadArgument
	case int32(unix.EPERM):
		return C.IOUSBHostCIMessageStatusNotPermitted
	case int32(unix.ECONNRESET):
		return C.IOUSBHostCIMessageStatusEndpointStopped
	default:
		return C.IOUSBHostCIMessageStatusError
	}
}

func darwinUSBIPStatusToCIStatus(status int32) int {
	return int(usbipStatusToDarwinCIStatus(status))
}
