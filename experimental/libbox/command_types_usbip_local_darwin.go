//go:build with_usbip && darwin && !ios && cgo

package libbox

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-usbip"
	E "github.com/sagernet/sing/common/exceptions"
)

func (c *CommandClient) NewUSBLocalProvider(handler USBLocalProviderHandler) (*USBLocalProviderManager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	manager := &USBLocalProviderManager{
		client:   c,
		handler:  handler,
		ctx:      ctx,
		cancel:   cancel,
		sessions: make(map[string]*usbDarwinLocalProviderSession),
		devices:  make(map[string]*usbDarwinProvidedDevice),
	}
	err := usbip.WatchLocalDevices(ctx, func() {
		manager.detachMissing()
		manager.notifyLocalDevicesChanged()
	})
	if err != nil {
		cancel()
		return nil, E.Cause(err, "watch local usb devices")
	}
	return manager, nil
}

type USBLocalProviderManager struct {
	client  *CommandClient
	handler USBLocalProviderHandler
	ctx     context.Context
	cancel  context.CancelFunc

	counter  atomic.Uint64
	access   sync.Mutex
	closed   bool
	sessions map[string]*usbDarwinLocalProviderSession
	devices  map[string]*usbDarwinProvidedDevice
}

type usbDarwinLocalProviderSession struct {
	tag     string
	session *USBProviderSession
}

type usbDarwinProvidedDevice struct {
	manager       *USBLocalProviderManager
	session       *usbDarwinLocalProviderSession
	local         usbip.LocalDevice
	info          *USBLocalProvidedDevice
	queueAccess   sync.Mutex
	queues        map[uint8]chan *USBURBRequest
	closed        bool
	closeOnce     sync.Once
	closeFinished chan struct{}
}

func (m *USBLocalProviderManager) ListDevices() (USBLocalDeviceInfoIterator, error) {
	devices, err := usbip.ListLocalDevices()
	if err != nil {
		return nil, err
	}
	out := make([]*USBLocalDeviceInfo, 0, len(devices))
	for i := range devices {
		out = append(out, usbLocalDeviceInfoFromUSBIP(devices[i]))
	}
	return newIterator(out), nil
}

func (m *USBLocalProviderManager) Attach(serverTag string, localDeviceID string) (*USBLocalProvidedDevice, error) {
	if serverTag == "" {
		return nil, E.New("missing usbip-server tag")
	}
	if localDeviceID == "" {
		return nil, E.New("missing local USB device id")
	}
	session, err := m.ensureSession(serverTag)
	if err != nil {
		return nil, err
	}
	localDevice, err := usbip.OpenLocalDevice(localDeviceID, false)
	if err != nil {
		return nil, err
	}
	deviceID := fmt.Sprintf("local-%d", m.counter.Add(1))
	localInfo := usbLocalDeviceInfoFromUSBIP(usbip.LocalDeviceInfo{
		StableID: localDevice.StableID(),
		Entry:    localDevice.Entry(),
	})
	descriptor := usbDeviceDescriptorFromLocalInfo(serverTag, deviceID, localInfo)
	provided := &USBLocalProvidedDevice{
		ServerTag:     serverTag,
		DeviceID:      deviceID,
		LocalDeviceID: localDevice.StableID(),
		Label:         localInfo.Product,
		VendorID:      localInfo.VendorID,
		ProductID:     localInfo.ProductID,
	}
	device := &usbDarwinProvidedDevice{
		manager:       m,
		session:       session,
		local:         localDevice,
		info:          provided,
		queues:        make(map[uint8]chan *USBURBRequest),
		closeFinished: make(chan struct{}),
	}
	m.access.Lock()
	if m.closed {
		m.access.Unlock()
		_ = localDevice.Close()
		return nil, os.ErrClosed
	}
	m.devices[deviceID] = device
	m.access.Unlock()
	err = session.session.AttachDevice(descriptor)
	if err != nil {
		m.removeDevice(deviceID)
		device.close(false)
		return nil, err
	}
	return provided, nil
}

func (m *USBLocalProviderManager) Detach(deviceID string) error {
	device := m.removeDevice(deviceID)
	if device == nil {
		return os.ErrNotExist
	}
	err := device.detach()
	device.close(false)
	return err
}

func (m *USBLocalProviderManager) Close() error {
	m.access.Lock()
	if m.closed {
		m.access.Unlock()
		return nil
	}
	m.closed = true
	devices := make([]*usbDarwinProvidedDevice, 0, len(m.devices))
	for _, device := range m.devices {
		devices = append(devices, device)
	}
	m.devices = make(map[string]*usbDarwinProvidedDevice)
	sessions := make([]*usbDarwinLocalProviderSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.sessions = make(map[string]*usbDarwinLocalProviderSession)
	m.access.Unlock()

	m.cancel()
	for _, device := range devices {
		device.close(false)
	}
	var err error
	for _, session := range sessions {
		err = E.Append(err, session.session.Close(), func(err error) error {
			return E.Cause(err, "close usb provider session ", session.tag)
		})
	}
	return err
}

func (m *USBLocalProviderManager) ensureSession(serverTag string) (*usbDarwinLocalProviderSession, error) {
	m.access.Lock()
	if m.closed {
		m.access.Unlock()
		return nil, os.ErrClosed
	}
	existing := m.sessions[serverTag]
	m.access.Unlock()
	if existing != nil {
		return existing, nil
	}
	session, err := m.client.ProvideUSBDevices(&usbDarwinLocalProviderStreamHandler{
		manager:   m,
		serverTag: serverTag,
	})
	if err != nil {
		return nil, err
	}
	wrapped := &usbDarwinLocalProviderSession{tag: serverTag, session: session}

	m.access.Lock()
	defer m.access.Unlock()
	if m.closed {
		_ = session.Close()
		return nil, os.ErrClosed
	}
	if existing = m.sessions[serverTag]; existing != nil {
		_ = session.Close()
		return existing, nil
	}
	m.sessions[serverTag] = wrapped
	return wrapped, nil
}

func (m *USBLocalProviderManager) removeDevice(deviceID string) *usbDarwinProvidedDevice {
	m.access.Lock()
	device := m.devices[deviceID]
	if device != nil {
		delete(m.devices, deviceID)
	}
	m.access.Unlock()
	return device
}

func (m *USBLocalProviderManager) device(deviceID string) *usbDarwinProvidedDevice {
	m.access.Lock()
	defer m.access.Unlock()
	return m.devices[deviceID]
}

func (m *USBLocalProviderManager) detachMissing() {
	devices, err := usbip.ListLocalDevices()
	if err != nil {
		m.notifySessionError("", E.Cause(err, "list local USB devices").Error())
		return
	}
	present := make(map[string]struct{}, len(devices))
	for i := range devices {
		present[devices[i].StableID] = struct{}{}
	}

	var stale []*usbDarwinProvidedDevice
	m.access.Lock()
	for deviceID, device := range m.devices {
		if _, ok := present[device.info.LocalDeviceID]; ok {
			continue
		}
		delete(m.devices, deviceID)
		stale = append(stale, device)
	}
	m.access.Unlock()

	for _, device := range stale {
		_ = device.detach()
		device.close(false)
		m.notifyDeviceError(device.info.ServerTag, device.info.DeviceID, "local USB device disconnected")
	}
}

func (m *USBLocalProviderManager) onURBRequest(request *USBURBRequest) {
	device := m.device(request.DeviceID)
	if device == nil {
		return
	}
	device.submit(request)
}

func (m *USBLocalProviderManager) onAbort(deviceID string, endpoint int32) {
	device := m.device(deviceID)
	if device == nil {
		return
	}
	err := device.local.AbortEndpoint(uint8(endpoint))
	if err != nil {
		m.notifyDeviceError(device.info.ServerTag, deviceID, err.Error())
	}
}

func (m *USBLocalProviderManager) onDeviceError(serverTag string, deviceID string, message string) {
	device := m.removeDevice(deviceID)
	if device != nil {
		device.close(false)
	}
	m.notifyDeviceError(serverTag, deviceID, message)
}

func (m *USBLocalProviderManager) onSessionError(serverTag string, message string) {
	var affected []*usbDarwinProvidedDevice
	m.access.Lock()
	delete(m.sessions, serverTag)
	for deviceID, device := range m.devices {
		if device.info.ServerTag != serverTag {
			continue
		}
		delete(m.devices, deviceID)
		affected = append(affected, device)
	}
	m.access.Unlock()

	for _, device := range affected {
		device.close(false)
	}
	m.notifySessionError(serverTag, message)
}

func (m *USBLocalProviderManager) notifyLocalDevicesChanged() {
	if m.handler != nil {
		m.handler.OnLocalDevicesChanged()
	}
}

func (m *USBLocalProviderManager) notifyDeviceError(serverTag string, deviceID string, message string) {
	if m.handler != nil {
		m.handler.OnDeviceError(serverTag, deviceID, message)
	}
}

func (m *USBLocalProviderManager) notifySessionError(serverTag string, message string) {
	if m.handler != nil {
		m.handler.OnSessionError(serverTag, message)
	}
}

type usbDarwinLocalProviderStreamHandler struct {
	manager   *USBLocalProviderManager
	serverTag string
}

func (h *usbDarwinLocalProviderStreamHandler) OnReady(deviceID string, busID string) {
}

func (h *usbDarwinLocalProviderStreamHandler) OnURBRequest(request *USBURBRequest) {
	h.manager.onURBRequest(request)
}

func (h *usbDarwinLocalProviderStreamHandler) OnAbort(deviceID string, endpoint int32) {
	h.manager.onAbort(deviceID, endpoint)
}

func (h *usbDarwinLocalProviderStreamHandler) OnError(deviceID string, message string) {
	if deviceID == "" {
		h.manager.onSessionError(h.serverTag, message)
		return
	}
	h.manager.onDeviceError(h.serverTag, deviceID, message)
}

func (d *usbDarwinProvidedDevice) submit(request *USBURBRequest) {
	endpoint := uint8(request.Endpoint)
	d.queueAccess.Lock()
	if d.closed {
		d.queueAccess.Unlock()
		d.sendResponse(usbURBErrorResponse(request))
		return
	}
	queue := d.queues[endpoint]
	if queue == nil {
		queue = make(chan *USBURBRequest, 64)
		d.queues[endpoint] = queue
		go d.runQueue(queue)
	}
	select {
	case queue <- request:
		d.queueAccess.Unlock()
	default:
		d.queueAccess.Unlock()
		d.sendResponse(usbURBErrorResponse(request))
	}
}

func (d *usbDarwinProvidedDevice) runQueue(queue <-chan *USBURBRequest) {
	for request := range queue {
		result := d.local.Submit(usbIPRequestFromLocalProvider(request))
		d.sendResponse(usbURBResponseFromUSBIP(request, result))
	}
}

func (d *usbDarwinProvidedDevice) detach() error {
	return d.session.session.DetachDevice(d.info.DeviceID)
}

func (d *usbDarwinProvidedDevice) close(detach bool) {
	d.closeOnce.Do(func() {
		defer close(d.closeFinished)
		d.queueAccess.Lock()
		d.closed = true
		for endpoint, queue := range d.queues {
			close(queue)
			delete(d.queues, endpoint)
		}
		d.queueAccess.Unlock()
		if detach {
			_ = d.detach()
		}
		_ = d.local.Close()
	})
	<-d.closeFinished
}

func (d *usbDarwinProvidedDevice) sendResponse(response *USBURBResponse) {
	err := d.session.session.SendURBResponse(response)
	if err != nil {
		d.manager.onSessionError(d.info.ServerTag, E.Cause(err, "send USB URB response").Error())
	}
}

func usbLocalDeviceInfoFromUSBIP(info usbip.LocalDeviceInfo) *USBLocalDeviceInfo {
	entry := info.Entry
	interfaces := make([]*USBSharedDeviceInterface, 0, len(entry.Interfaces))
	for _, deviceInterface := range entry.Interfaces {
		interfaces = append(interfaces, &USBSharedDeviceInterface{
			InterfaceClass:    int32(deviceInterface.BInterfaceClass),
			InterfaceSubClass: int32(deviceInterface.BInterfaceSubClass),
			InterfaceProtocol: int32(deviceInterface.BInterfaceProtocol),
		})
	}
	return &USBLocalDeviceInfo{
		StableID:           info.StableID,
		BusID:              entry.Info.BusIDString(),
		Backend:            int32(info.Backend),
		BusNum:             int32(entry.Info.BusNum),
		DevNum:             int32(entry.Info.DevNum),
		Speed:              int32(entry.Info.Speed),
		VendorID:           int32(entry.Info.IDVendor),
		ProductID:          int32(entry.Info.IDProduct),
		BCDDevice:          int32(entry.Info.BCDDevice),
		DeviceClass:        int32(entry.Info.BDeviceClass),
		DeviceSubClass:     int32(entry.Info.BDeviceSubClass),
		DeviceProtocol:     int32(entry.Info.BDeviceProtocol),
		ConfigurationValue: int32(entry.Info.BConfigurationValue),
		NumConfigurations:  int32(entry.Info.BNumConfigurations),
		Serial:             entry.Serial,
		Product:            entry.Product,
		interfaces:         interfaces,
	}
}

func usbDeviceDescriptorFromLocalInfo(serverTag string, deviceID string, info *USBLocalDeviceInfo) *USBDeviceDescriptor {
	descriptor := &USBDeviceDescriptor{
		ServerTag:          serverTag,
		DeviceID:           deviceID,
		BusNum:             info.BusNum,
		DevNum:             info.DevNum,
		Speed:              info.Speed,
		VendorID:           info.VendorID,
		ProductID:          info.ProductID,
		BCDDevice:          info.BCDDevice,
		DeviceClass:        info.DeviceClass,
		DeviceSubClass:     info.DeviceSubClass,
		DeviceProtocol:     info.DeviceProtocol,
		ConfigurationValue: info.ConfigurationValue,
		NumConfigurations:  info.NumConfigurations,
		Serial:             info.Serial,
		Product:            info.Product,
	}
	for _, deviceInterface := range info.interfaces {
		descriptor.interfaces = append(descriptor.interfaces, &daemon.USBInterface{
			InterfaceClass:    uint32(deviceInterface.InterfaceClass),
			InterfaceSubClass: uint32(deviceInterface.InterfaceSubClass),
			InterfaceProtocol: uint32(deviceInterface.InterfaceProtocol),
		})
	}
	return descriptor
}

func usbIPRequestFromLocalProvider(request *USBURBRequest) usbip.URBRequest {
	endpoint := uint8(request.Endpoint)
	direction := usbip.USBIPDirOut
	if request.DirectionIn {
		direction = usbip.USBIPDirIn
	}
	var setup [8]byte
	copy(setup[:], request.Setup)
	buffer := request.OutData
	if request.DirectionIn {
		buffer = make([]byte, max(0, int(request.TransferBufferLength)))
	}
	isoPackets := make([]usbip.IsoPacketDescriptor, 0, request.IsoPacketCount())
	for i := int32(0); i < request.IsoPacketCount(); i++ {
		packet := request.GetIsoPacket(i)
		if packet == nil {
			continue
		}
		isoPackets = append(isoPackets, usbip.IsoPacketDescriptor{
			Offset:       packet.Offset,
			Length:       packet.Length,
			ActualLength: packet.ActualLength,
			Status:       packet.Status,
		})
	}
	command := usbip.SubmitCommand{
		Header: usbip.DataHeader{
			Command:   usbip.CmdSubmit,
			SeqNum:    uint32(request.Seq),
			Direction: direction,
			Endpoint:  uint32(endpoint & 0x0f),
		},
		TransferFlags:        request.TransferFlags,
		TransferBufferLength: request.TransferBufferLength,
		StartFrame:           request.StartFrame,
		NumberOfPackets:      request.NumberOfPackets,
		Interval:             request.Interval,
		Setup:                setup,
		Buffer:               buffer,
		IsoPackets:           isoPackets,
	}
	return usbip.URBRequest{
		Command:    command,
		Endpoint:   endpoint,
		Buffer:     buffer,
		IsoPackets: isoPackets,
	}
}

func usbURBResponseFromUSBIP(request *USBURBRequest, result usbip.URBResponse) *USBURBResponse {
	if result.Error != nil {
		return usbURBErrorResponse(request)
	}
	response := NewUSBURBResponse(request.DeviceID, request.Seq)
	response.Status = result.Status
	response.ActualLength = result.ActualLength
	for _, packet := range result.IsoPackets {
		response.AddIsoPacket(packet.Offset, packet.Length, packet.ActualLength, packet.Status)
	}
	if request.DirectionIn && len(result.Buffer) > 0 {
		if request.NumberOfPackets > 0 {
			response.InData = result.Buffer
		} else {
			actual := int(result.ActualLength)
			if actual < 0 {
				actual = 0
			}
			response.InData = result.Buffer[:min(actual, len(result.Buffer))]
		}
	}
	return response
}

func usbURBErrorResponse(request *USBURBRequest) *USBURBResponse {
	response := NewUSBURBResponse(request.DeviceID, request.Seq)
	response.Status = -5
	return response
}
