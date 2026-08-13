//go:build with_usbip && (linux || (darwin && cgo) || windows)

package main

import (
	"context"
	"errors"
	"io"
	"maps"
	"os"
	"os/signal"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-usbip"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	usbipShareDrainTimeout   = 2 * time.Second
	usbipShareQueueDepth     = 64
	usbipShareMaxDeviceIDTry = 9
	usbipShareStatusEIO      = -5
)

func runAPIUsbipShare(args []string) error {
	if commandAPIUsbipShareFlagAll == (len(args) > 0) {
		return E.New("either --all or one or more device selectors are required")
	}
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()

	sessionCtx, cancelSession := context.WithCancel(globalCtx)
	defer cancelSession()
	statusCtx, cancelStatus := context.WithCancel(sessionCtx)
	defer cancelStatus()
	watchCtx, cancelWatch := context.WithCancel(sessionCtx)
	defer cancelWatch()

	statusStream, err := client.SubscribeUSBIPServerStatus(statusCtx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	statusUpdate, err := statusStream.Recv()
	if err != nil {
		return err
	}
	server, err := resolveUsbipServer(statusUpdate.GetServers(), commandAPIUsbipShareFlagService)
	if err != nil {
		return err
	}

	localDevices, err := listLocalUSBDevices()
	if err != nil {
		return err
	}
	selected, err := selectSharedUSBDevices(localDevices, args)
	if err != nil {
		return err
	}

	stream, err := client.ProvideUSBDevices(sessionCtx)
	if err != nil {
		return err
	}
	session := &usbipShareSession{
		ctx:       sessionCtx,
		serverTag: server.GetServerTag(),
		capture:   commandAPIUsbipShareFlagCapture,
		shareAll:  commandAPIUsbipShareFlagAll,
		stream:    stream,
		devices:   make(map[string]*usbipSharedDevice),
		states:    make(map[string]daemon.USBDeviceState),
		failed:    make(map[string]struct{}),
	}

	signalChan := make(chan os.Signal, 2)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signalChan)

	receiveDone := make(chan struct{})
	go func() {
		defer close(receiveDone)
		session.receive()
	}()
	go session.readStatus(statusStream)

	for _, device := range selected {
		shareErr := session.share(device, "")
		if shareErr != nil {
			cancelStatus()
			session.closeAll()
			return shareErr
		}
	}

	// The darwin watcher fires once at registration, so it is registered only after
	// the initial devices are shared.
	err = usbip.WatchLocalDevices(watchCtx, session.onLocalDevicesChanged)
	if err != nil {
		cancelStatus()
		session.closeAll()
		return E.Cause(err, "watch local USB devices")
	}

	select {
	case <-signalChan:
		cancelWatch()
		cancelStatus()
		session.teardown(signalChan, receiveDone)
		return nil
	case <-receiveDone:
		cancelWatch()
		cancelStatus()
		return session.abort()
	}
}

func selectSharedUSBDevices(devices []usbip.LocalDeviceInfo, selectors []string) ([]usbip.LocalDeviceInfo, error) {
	if len(selectors) == 0 {
		return devices, nil
	}
	selected := make([]usbip.LocalDeviceInfo, 0, len(selectors))
	for _, selector := range selectors {
		device, err := resolveLocalUSBDevice(devices, selector)
		if err != nil {
			return nil, err
		}
		busID := device.Entry.Info.BusIDString()
		if slices.ContainsFunc(selected, func(it usbip.LocalDeviceInfo) bool {
			return it.Entry.Info.BusIDString() == busID
		}) {
			continue
		}
		selected = append(selected, device)
	}
	return selected, nil
}

type usbipDeviceIdentity struct {
	vendorID  uint16
	productID uint16
	serial    string
	product   string
}

func usbipIdentityOf(device usbip.LocalDeviceInfo) usbipDeviceIdentity {
	return usbipDeviceIdentity{
		vendorID:  device.Entry.Info.IDVendor,
		productID: device.Entry.Info.IDProduct,
		serial:    device.Entry.Serial,
		product:   device.Entry.Product,
	}
}

type usbipReconnectIntent struct {
	identity usbipDeviceIdentity
	deviceID string
}

type usbipShareSession struct {
	ctx       context.Context
	serverTag string
	capture   bool
	shareAll  bool
	stream    daemon.StartedService_ProvideUSBDevicesClient

	sendAccess  sync.Mutex
	printAccess sync.Mutex

	access  sync.Mutex
	closed  bool
	devices map[string]*usbipSharedDevice
	intents []usbipReconnectIntent
	states  map[string]daemon.USBDeviceState
	failed  map[string]struct{}

	terminalError  error
	terminalReason string
}

type usbipSharedDevice struct {
	session    *usbipShareSession
	info       usbip.LocalDeviceInfo
	local      usbip.LocalDevice
	localBusID string
	identity   usbipDeviceIdentity

	deviceID     string
	attempt      int
	seenInStatus bool

	queueAccess   sync.Mutex
	queues        map[uint8]chan *daemon.USBURBRequest
	closed        bool
	closeOnce     sync.Once
	closeFinished chan struct{}
}

func (s *usbipShareSession) share(info usbip.LocalDeviceInfo, preferredDeviceID string) error {
	busID := info.Entry.Info.BusIDString()
	local, err := usbip.OpenLocalDevice(busID, s.capture)
	if err != nil {
		s.access.Lock()
		s.failed[busID] = struct{}{}
		s.access.Unlock()
		s.printEvent("error", busID, err.Error())
		s.writeOpenHint(err)
		if usbipShareFatalOpenError(err) {
			return err
		}
		return nil
	}
	deviceID := preferredDeviceID
	if deviceID == "" {
		deviceID = busID
	}
	device := &usbipSharedDevice{
		session:       s,
		info:          info,
		local:         local,
		localBusID:    busID,
		identity:      usbipIdentityOf(info),
		deviceID:      deviceID,
		attempt:       1,
		queues:        make(map[uint8]chan *daemon.USBURBRequest),
		closeFinished: make(chan struct{}),
	}
	s.access.Lock()
	if s.closed {
		s.access.Unlock()
		_ = local.Close()
		return nil
	}
	_, taken := s.devices[deviceID]
	if taken {
		s.access.Unlock()
		device.close()
		s.printEvent("error", deviceID, "device id already in use by this session")
		return nil
	}
	s.devices[deviceID] = device
	s.access.Unlock()
	sendErr := s.send(usbipAttachMessage(s.serverTag, deviceID, info))
	if sendErr != nil {
		s.removeDevice(deviceID)
		device.close()
		s.printEvent("error", deviceID, sendErr.Error())
	}
	return nil
}

func (s *usbipShareSession) send(message *daemon.USBProviderMessage) error {
	s.sendAccess.Lock()
	defer s.sendAccess.Unlock()
	return s.stream.Send(message)
}

func (s *usbipShareSession) detach(deviceID string) {
	_ = s.send(&daemon.USBProviderMessage{Message: &daemon.USBProviderMessage_Detach{
		Detach: &daemon.USBDeviceDetach{DeviceId: deviceID},
	}})
}

func (s *usbipShareSession) device(deviceID string) *usbipSharedDevice {
	s.access.Lock()
	defer s.access.Unlock()
	return s.devices[deviceID]
}

func (s *usbipShareSession) removeDevice(deviceID string) *usbipSharedDevice {
	s.access.Lock()
	defer s.access.Unlock()
	device := s.devices[deviceID]
	if device != nil {
		delete(s.devices, deviceID)
		delete(s.states, deviceID)
	}
	return device
}

func (s *usbipShareSession) receive() {
	for {
		message, err := s.stream.Recv()
		if err != nil {
			if s.ctx.Err() == nil {
				if err == io.EOF {
					s.terminalError = E.New("provider stream closed by the API service")
				} else {
					s.terminalError = err
				}
				s.terminalReason = "stream-closed"
			}
			return
		}
		switch body := message.GetMessage().(type) {
		case *daemon.USBServerMessage_Ready:
			s.onReady(body.Ready)
		case *daemon.USBServerMessage_UrbRequest:
			device := s.device(body.UrbRequest.GetDeviceId())
			if device != nil {
				device.submit(body.UrbRequest)
			}
		case *daemon.USBServerMessage_Abort:
			s.onAbort(body.Abort)
		case *daemon.USBServerMessage_Error:
			if s.onError(body.Error) {
				return
			}
		}
	}
}

func (s *usbipShareSession) onReady(ready *daemon.USBDeviceReady) {
	device := s.device(ready.GetDeviceId())
	if device == nil {
		return
	}
	busID := ready.GetDeviceId()
	if busID != device.localBusID {
		busID = F.ToString(busID, " (local ", device.localBusID, ")")
	}
	s.printEvent("shared", busID,
		usbipVendorProduct(device.identity.vendorID, device.identity.productID),
		device.identity.product,
	)
}

func (s *usbipShareSession) onAbort(abort *daemon.USBEndpointAbort) {
	device := s.device(abort.GetDeviceId())
	if device == nil {
		return
	}
	err := device.local.AbortEndpoint(uint8(abort.GetEndpoint()))
	if err != nil {
		s.printEvent("error", abort.GetDeviceId(), err.Error())
	}
}

func (s *usbipShareSession) onError(failure *daemon.USBError) bool {
	deviceID := failure.GetDeviceId()
	message := failure.GetMessage()
	if deviceID == "" ||
		strings.Contains(message, "usbip-server not found:") ||
		strings.Contains(message, "is not a dynamic usbip-server") {
		s.terminalError = E.New(message)
		s.terminalReason = "server-error"
		return true
	}
	if strings.Contains(message, "dynamic device already provided:") {
		s.retryDeviceID(deviceID, message)
		return false
	}
	device := s.removeDevice(deviceID)
	if device != nil {
		device.close()
	}
	s.printEvent("error", deviceID, message)
	return false
}

func (s *usbipShareSession) retryDeviceID(deviceID string, message string) {
	s.access.Lock()
	device := s.devices[deviceID]
	if device == nil || device.attempt >= usbipShareMaxDeviceIDTry {
		s.access.Unlock()
		if device != nil {
			s.removeDevice(deviceID)
			device.close()
			s.printEvent("error", deviceID, message)
		}
		return
	}
	delete(s.devices, deviceID)
	delete(s.states, deviceID)
	device.attempt++
	device.deviceID = F.ToString(device.localBusID, "#", device.attempt)
	s.devices[device.deviceID] = device
	retryID := device.deviceID
	s.access.Unlock()
	sendErr := s.send(usbipAttachMessage(s.serverTag, retryID, device.info))
	if sendErr != nil {
		s.removeDevice(retryID)
		device.close()
		s.printEvent("error", retryID, sendErr.Error())
	}
}

func (s *usbipShareSession) readStatus(stream daemon.StartedService_SubscribeUSBIPServerStatusClient) {
	for {
		update, err := stream.Recv()
		if err != nil {
			return
		}
		index := slices.IndexFunc(update.GetServers(), func(it *daemon.USBIPServerStatus) bool {
			return it.GetServerTag() == s.serverTag
		})
		if index == -1 {
			continue
		}
		snapshot := make(map[string]daemon.USBDeviceState)
		for _, device := range update.GetServers()[index].GetDevices() {
			snapshot[device.GetBusId()] = device.GetState()
		}
		s.applyStatusSnapshot(snapshot)
	}
}

func (s *usbipShareSession) applyStatusSnapshot(snapshot map[string]daemon.USBDeviceState) {
	type transition struct {
		deviceID string
		verb     string
	}
	var (
		transitions []transition
		lost        []*usbipSharedDevice
	)
	s.access.Lock()
	for deviceID, device := range s.devices {
		state, present := snapshot[deviceID]
		if !present {
			if device.seenInStatus {
				device.seenInStatus = false
				delete(s.states, deviceID)
				lost = append(lost, device)
			}
			continue
		}
		previous, tracked := s.states[deviceID]
		s.states[deviceID] = state
		if !device.seenInStatus {
			device.seenInStatus = true
			continue
		}
		if !tracked || previous == state {
			continue
		}
		switch {
		case state == daemon.USBDeviceState_USB_DEVICE_STATE_ATTACHED:
			transitions = append(transitions, transition{deviceID: deviceID, verb: "attached"})
		case previous == daemon.USBDeviceState_USB_DEVICE_STATE_ATTACHED:
			transitions = append(transitions, transition{deviceID: deviceID, verb: "released"})
		}
	}
	s.access.Unlock()

	for _, item := range transitions {
		s.printEvent(item.verb, item.deviceID)
	}
	for _, device := range lost {
		s.printEvent("detached", device.deviceID, "server-error")
		sendErr := s.send(usbipAttachMessage(s.serverTag, device.deviceID, device.info))
		if sendErr != nil {
			s.removeDevice(device.deviceID)
			device.close()
			s.printEvent("error", device.deviceID, sendErr.Error())
		}
	}
}

func (s *usbipShareSession) onLocalDevicesChanged() {
	devices, err := listLocalUSBDevices()
	if err != nil {
		return
	}
	present := make(map[string]struct{}, len(devices))
	for _, device := range devices {
		present[device.Entry.Info.BusIDString()] = struct{}{}
	}

	var vanished []*usbipSharedDevice
	s.access.Lock()
	if s.closed {
		s.access.Unlock()
		return
	}
	// Capturing a device on windows rewrites its reported vendor and product id, so
	// presence is decided by bus id alone; the identity tuple is only used to match a
	// replugged device back to the export it replaces.
	for deviceID, device := range s.devices {
		_, found := present[device.localBusID]
		if found {
			continue
		}
		delete(s.devices, deviceID)
		delete(s.states, deviceID)
		vanished = append(vanished, device)
	}
	for _, device := range vanished {
		s.intents = append(s.intents, usbipReconnectIntent{identity: device.identity, deviceID: device.deviceID})
	}
	for busID := range s.failed {
		_, found := present[busID]
		if !found {
			delete(s.failed, busID)
		}
	}
	s.access.Unlock()

	for _, device := range vanished {
		s.detach(device.deviceID)
		device.close()
		s.printEvent("detached", device.deviceID, "unplugged")
	}
	s.reshare(devices)
}

func (s *usbipShareSession) reshare(devices []usbip.LocalDeviceInfo) {
	s.access.Lock()
	if s.closed {
		s.access.Unlock()
		return
	}
	shared := make(map[string]struct{}, len(s.devices))
	for _, device := range s.devices {
		shared[device.localBusID] = struct{}{}
	}
	intents := slices.Clone(s.intents)
	failed := maps.Clone(s.failed)
	s.access.Unlock()

	available := common.Filter(devices, func(it usbip.LocalDeviceInfo) bool {
		_, found := shared[it.Entry.Info.BusIDString()]
		return !found
	})
	claimed := make(map[string]struct{})
	var pending []usbipReconnectIntent
	for _, intent := range intents {
		matches := common.Filter(available, func(it usbip.LocalDeviceInfo) bool {
			_, found := claimed[it.Entry.Info.BusIDString()]
			return !found && usbipIdentityOf(it) == intent.identity
		})
		if len(matches) != 1 {
			pending = append(pending, intent)
			continue
		}
		claimed[matches[0].Entry.Info.BusIDString()] = struct{}{}
		s.consumeIntent(intent)
		_ = s.share(matches[0], intent.deviceID)
	}
	if !s.shareAll {
		return
	}
	for _, device := range available {
		busID := device.Entry.Info.BusIDString()
		_, claimedHere := claimed[busID]
		_, failedBefore := failed[busID]
		if claimedHere || failedBefore {
			continue
		}
		if slices.ContainsFunc(pending, func(it usbipReconnectIntent) bool {
			return it.identity == usbipIdentityOf(device)
		}) {
			continue
		}
		_ = s.share(device, "")
	}
}

func (s *usbipShareSession) consumeIntent(intent usbipReconnectIntent) {
	s.access.Lock()
	defer s.access.Unlock()
	index := slices.Index(s.intents, intent)
	if index != -1 {
		s.intents = slices.Delete(s.intents, index, index+1)
	}
}

func (s *usbipShareSession) takeDevices() []*usbipSharedDevice {
	s.access.Lock()
	defer s.access.Unlock()
	s.closed = true
	s.intents = nil
	devices := slices.Collect(maps.Values(s.devices))
	s.devices = make(map[string]*usbipSharedDevice)
	clear(s.states)
	slices.SortFunc(devices, func(a *usbipSharedDevice, b *usbipSharedDevice) int {
		return compareBusID(a.deviceID, b.deviceID)
	})
	return devices
}

func (s *usbipShareSession) teardown(signalChan <-chan os.Signal, receiveDone <-chan struct{}) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		devices := s.takeDevices()
		for _, device := range devices {
			s.detach(device.deviceID)
		}
		_ = s.stream.CloseSend()
		timer := time.NewTimer(usbipShareDrainTimeout)
		defer timer.Stop()
		select {
		case <-receiveDone:
		case <-timer.C:
		}
		for _, device := range devices {
			device.close()
		}
		for _, device := range devices {
			s.printEvent("detached", device.deviceID, "signal")
		}
	}()
	select {
	case <-done:
	case <-signalChan:
		if runtime.GOOS == "windows" {
			os.Stderr.WriteString("interrupted while stopping, USB devices may remain captured\n")
		}
		os.Exit(130)
	}
}

func (s *usbipShareSession) abort() error {
	reason := s.terminalReason
	if reason == "" {
		reason = "stream-closed"
	}
	for _, device := range s.takeDevices() {
		device.close()
		s.printEvent("detached", device.deviceID, reason)
	}
	if s.terminalError == nil {
		return E.New("provider stream closed")
	}
	return s.terminalError
}

func (s *usbipShareSession) closeAll() {
	for _, device := range s.takeDevices() {
		device.close()
	}
}

func (s *usbipShareSession) printEvent(verb string, busID string, fields ...string) {
	var builder strings.Builder
	builder.WriteString(padUSBIPCell(verb, 9))
	builder.WriteString("  ")
	builder.WriteString(padUSBIPCell(busID, 10))
	for _, field := range fields {
		builder.WriteString("  ")
		if field == "" {
			field = "-"
		}
		builder.WriteString(field)
	}
	line := strings.TrimRight(builder.String(), " ") + "\n"
	s.printAccess.Lock()
	defer s.printAccess.Unlock()
	os.Stdout.WriteString(line)
}

func (s *usbipShareSession) writeOpenHint(err error) {
	if errors.Is(err, os.ErrPermission) {
		os.Stderr.WriteString("hint: run sing-box with elevated privileges to access this device\n")
		return
	}
	if !s.capture {
		os.Stderr.WriteString("hint: retry with --capture to take the device from the driver using it (requires root or Administrator)\n")
	}
}

// The VBoxUSB driver installation runs before any device is touched, so these
// failures apply to every device in the session.
func usbipShareFatalOpenError(err error) bool {
	message := err.Error()
	return strings.Contains(message, "enable SeLoadDriverPrivilege") ||
		strings.Contains(message, "install VBoxUSB drivers")
}

func (d *usbipSharedDevice) submit(request *daemon.USBURBRequest) {
	endpoint := uint8(request.GetEndpoint())
	d.queueAccess.Lock()
	if d.closed {
		d.queueAccess.Unlock()
		d.sendResponse(usbipURBErrorResponse(request))
		return
	}
	queue := d.queues[endpoint]
	if queue == nil {
		queue = make(chan *daemon.USBURBRequest, usbipShareQueueDepth)
		d.queues[endpoint] = queue
		go d.runQueue(queue)
	}
	select {
	case queue <- request:
		d.queueAccess.Unlock()
	default:
		d.queueAccess.Unlock()
		d.sendResponse(usbipURBErrorResponse(request))
	}
}

func (d *usbipSharedDevice) runQueue(queue <-chan *daemon.USBURBRequest) {
	for request := range queue {
		result := d.local.Submit(usbipURBRequestFromProto(request))
		d.sendResponse(usbipURBResponseToProto(request, result))
	}
}

func (d *usbipSharedDevice) sendResponse(response *daemon.USBURBResponse) {
	_ = d.session.send(&daemon.USBProviderMessage{Message: &daemon.USBProviderMessage_UrbResponse{UrbResponse: response}})
}

func (d *usbipSharedDevice) close() {
	d.closeOnce.Do(func() {
		defer close(d.closeFinished)
		d.queueAccess.Lock()
		d.closed = true
		for endpoint, queue := range d.queues {
			close(queue)
			delete(d.queues, endpoint)
		}
		d.queueAccess.Unlock()
		_ = d.local.Close()
	})
	<-d.closeFinished
}

func usbipAttachMessage(serverTag string, deviceID string, info usbip.LocalDeviceInfo) *daemon.USBProviderMessage {
	entry := info.Entry
	return &daemon.USBProviderMessage{Message: &daemon.USBProviderMessage_Attach{Attach: &daemon.USBDeviceAttach{
		ServerTag: serverTag,
		Descriptor_: &daemon.USBDeviceDescriptor{
			DeviceId:           deviceID,
			BusNum:             entry.Info.BusNum,
			DevNum:             entry.Info.DevNum,
			Speed:              entry.Info.Speed,
			VendorId:           uint32(entry.Info.IDVendor),
			ProductId:          uint32(entry.Info.IDProduct),
			BcdDevice:          uint32(entry.Info.BCDDevice),
			DeviceClass:        uint32(entry.Info.BDeviceClass),
			DeviceSubClass:     uint32(entry.Info.BDeviceSubClass),
			DeviceProtocol:     uint32(entry.Info.BDeviceProtocol),
			ConfigurationValue: uint32(entry.Info.BConfigurationValue),
			NumConfigurations:  uint32(entry.Info.BNumConfigurations),
			Interfaces: common.Map(entry.Interfaces, func(it usbip.DeviceInterface) *daemon.USBInterface {
				return &daemon.USBInterface{
					InterfaceClass:    uint32(it.BInterfaceClass),
					InterfaceSubClass: uint32(it.BInterfaceSubClass),
					InterfaceProtocol: uint32(it.BInterfaceProtocol),
				}
			}),
			Serial:  entry.Serial,
			Product: entry.Product,
		},
	}}}
}

func usbipURBRequestFromProto(request *daemon.USBURBRequest) usbip.URBRequest {
	endpoint := uint8(request.GetEndpoint())
	direction := usbip.USBIPDirOut
	buffer := request.GetOutData()
	if request.GetDirectionIn() {
		direction = usbip.USBIPDirIn
		buffer = make([]byte, request.GetTransferBufferLength())
	}
	var setup [8]byte
	copy(setup[:], request.GetSetup())
	isoPackets := common.Map(request.GetIsoPackets(), func(it *daemon.USBIsoPacket) usbip.IsoPacketDescriptor {
		return usbip.IsoPacketDescriptor{
			Offset:       it.GetOffset(),
			Length:       it.GetLength(),
			ActualLength: it.GetActualLength(),
			Status:       it.GetStatus(),
		}
	})
	return usbip.URBRequest{
		Command: usbip.SubmitCommand{
			Header: usbip.DataHeader{
				Command:   usbip.CmdSubmit,
				SeqNum:    uint32(request.GetSeq()),
				Direction: direction,
				Endpoint:  uint32(endpoint & 0x0f),
			},
			TransferFlags:        int32(request.GetTransferFlags()),
			TransferBufferLength: int32(request.GetTransferBufferLength()),
			StartFrame:           request.GetStartFrame(),
			NumberOfPackets:      request.GetNumberOfPackets(),
			Interval:             request.GetInterval(),
			Setup:                setup,
			Buffer:               buffer,
			IsoPackets:           isoPackets,
		},
		Endpoint:   endpoint,
		Buffer:     buffer,
		IsoPackets: isoPackets,
	}
}

func usbipURBResponseToProto(request *daemon.USBURBRequest, result usbip.URBResponse) *daemon.USBURBResponse {
	if result.Error != nil {
		return usbipURBErrorResponse(request)
	}
	response := &daemon.USBURBResponse{
		DeviceId:     request.GetDeviceId(),
		Seq:          request.GetSeq(),
		Status:       result.Status,
		ActualLength: result.ActualLength,
		IsoPackets: common.Map(result.IsoPackets, func(it usbip.IsoPacketDescriptor) *daemon.USBIsoPacket {
			return &daemon.USBIsoPacket{
				Offset:       it.Offset,
				Length:       it.Length,
				ActualLength: it.ActualLength,
				Status:       it.Status,
			}
		}),
	}
	if request.GetDirectionIn() && len(result.Buffer) > 0 {
		if request.GetNumberOfPackets() > 0 {
			response.InData = result.Buffer
		} else {
			response.InData = result.Buffer[:min(max(int(result.ActualLength), 0), len(result.Buffer))]
		}
	}
	return response
}

func usbipURBErrorResponse(request *daemon.USBURBRequest) *daemon.USBURBResponse {
	return &daemon.USBURBResponse{
		DeviceId: request.GetDeviceId(),
		Seq:      request.GetSeq(),
		Status:   usbipShareStatusEIO,
	}
}
