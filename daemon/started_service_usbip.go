//go:build with_usbip && (linux || (darwin && cgo) || windows)

package daemon

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-usbip"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *StartedService) ProvideUSBDevices(server grpc.BidiStreamingServer[USBProviderMessage, USBServerMessage]) error {
	ctx := server.Context()
	err := s.waitForStarted(ctx)
	if err != nil {
		return err
	}
	s.serviceAccess.RLock()
	instance := s.instance
	s.serviceAccess.RUnlock()
	if instance == nil {
		return E.New("service not started")
	}
	serviceManager := service.FromContext[adapter.ServiceManager](instance.ctx)
	if serviceManager == nil {
		return E.New("missing service manager")
	}

	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var sendAccess sync.Mutex
	send := func(message *USBServerMessage) error {
		sendAccess.Lock()
		defer sendAccess.Unlock()
		return server.Send(message)
	}

	var devicesAccess sync.Mutex
	devices := make(map[string]*usbProvidedDevice)
	defer func() {
		devicesAccess.Lock()
		for _, device := range devices {
			device.close()
		}
		devicesAccess.Unlock()
	}()

	for {
		message, recvErr := server.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				return nil
			}
			return recvErr
		}
		switch body := message.GetMessage().(type) {
		case *USBProviderMessage_Attach:
			attach := body.Attach
			deviceID := attach.GetDescriptor_().GetDeviceId()
			device, addErr := addUSBDevice(sessionCtx, serviceManager, send, attach)
			if addErr != nil {
				_ = send(&USBServerMessage{Message: &USBServerMessage_Error{Error: &USBError{
					DeviceId: deviceID,
					Message:  addErr.Error(),
				}}})
				continue
			}
			devicesAccess.Lock()
			previous, replaced := devices[deviceID]
			devices[deviceID] = device
			devicesAccess.Unlock()
			if replaced {
				previous.close()
			}
			_ = send(&USBServerMessage{Message: &USBServerMessage_Ready{Ready: &USBDeviceReady{
				DeviceId: deviceID,
				BusId:    device.busID,
			}}})
		case *USBProviderMessage_Detach:
			deviceID := body.Detach.GetDeviceId()
			devicesAccess.Lock()
			device, found := devices[deviceID]
			if found {
				delete(devices, deviceID)
			}
			devicesAccess.Unlock()
			if found {
				device.close()
			}
		case *USBProviderMessage_UrbResponse:
			response := body.UrbResponse
			devicesAccess.Lock()
			device, found := devices[response.GetDeviceId()]
			devicesAccess.Unlock()
			if found {
				device.deliver(response)
			}
		}
	}
}

func (s *StartedService) SubscribeUSBIPServerStatus(
	_ *emptypb.Empty,
	server grpc.ServerStreamingServer[USBIPServerStatusUpdate],
) error {
	err := s.waitForStarted(server.Context())
	if err != nil {
		return err
	}
	s.serviceAccess.RLock()
	instance := s.instance
	s.serviceAccess.RUnlock()
	if instance == nil {
		return E.New("service not started")
	}
	serviceManager := service.FromContext[adapter.ServiceManager](instance.ctx)
	if serviceManager == nil {
		return status.Error(codes.FailedPrecondition, "service manager not available")
	}

	type usbipServer struct {
		tag      string
		provider adapter.USBIPDynamicServer
	}
	var servers []usbipServer
	for _, serverService := range serviceManager.Services() {
		provider, isDynamic := serverService.(adapter.USBIPDynamicServer)
		if !isDynamic {
			continue
		}
		servers = append(servers, usbipServer{tag: serverService.Tag(), provider: provider})
	}
	if len(servers) == 0 {
		return status.Error(codes.NotFound, "no usbip-server found")
	}

	type taggedStatus struct {
		tag     string
		devices []usbip.ControlDeviceInfo
	}
	updates := make(chan taggedStatus, len(servers))
	ctx, cancel := context.WithCancel(server.Context())
	defer cancel()

	var waitGroup sync.WaitGroup
	for _, srv := range servers {
		// sing-usbip invokes the SubscribeDevices listener while holding the
		// ledger's broadcast lock, so it must never block.
		latest := make(chan []usbip.ControlDeviceInfo, 1)
		waitGroup.Add(1)
		go func(provider adapter.USBIPDynamicServer) {
			defer waitGroup.Done()
			provider.SubscribeDevices(ctx, func(devices []usbip.ControlDeviceInfo) {
				sendLatestUSBSnapshot(latest, devices)
			})
		}(srv.provider)
		waitGroup.Add(1)
		go func(tag string) {
			defer waitGroup.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case devices := <-latest:
					select {
					case updates <- taggedStatus{tag: tag, devices: devices}:
					case <-ctx.Done():
						return
					}
				}
			}
		}(srv.tag)
	}

	go func() {
		waitGroup.Wait()
		close(updates)
	}()

	var tags []string
	deviceStates := make(map[string][]usbip.ControlDeviceInfo, len(servers))
	for update := range updates {
		if _, exists := deviceStates[update.tag]; !exists {
			tags = append(tags, update.tag)
		}
		deviceStates[update.tag] = update.devices
		protoServers := make([]*USBIPServerStatus, 0, len(deviceStates))
		for _, tag := range tags {
			protoServers = append(protoServers, &USBIPServerStatus{
				ServerTag: tag,
				Devices:   usbSharedDevicesToProto(deviceStates[tag]),
			})
		}
		sendErr := server.Send(&USBIPServerStatusUpdate{Servers: protoServers})
		if sendErr != nil {
			return sendErr
		}
	}
	return nil
}

func sendLatestUSBSnapshot(slot chan []usbip.ControlDeviceInfo, devices []usbip.ControlDeviceInfo) {
	select {
	case slot <- devices:
		return
	default:
	}
	select {
	case <-slot:
	default:
	}
	select {
	case slot <- devices:
	default:
	}
}

func usbSharedDevicesToProto(devices []usbip.ControlDeviceInfo) []*USBSharedDevice {
	if len(devices) == 0 {
		return nil
	}
	out := make([]*USBSharedDevice, 0, len(devices))
	for _, device := range devices {
		interfaces := make([]*USBInterface, 0, len(device.Interfaces))
		for _, deviceInterface := range device.Interfaces {
			interfaces = append(interfaces, &USBInterface{
				InterfaceClass:    uint32(deviceInterface.Class),
				InterfaceSubClass: uint32(deviceInterface.SubClass),
				InterfaceProtocol: uint32(deviceInterface.Protocol),
			})
		}
		out = append(out, &USBSharedDevice{
			Descriptor_: &USBDeviceDescriptor{
				DeviceId:           device.BusID,
				BusNum:             device.BusNum,
				DevNum:             device.DevNum,
				Speed:              device.Speed,
				VendorId:           uint32(device.VendorID),
				ProductId:          uint32(device.ProductID),
				BcdDevice:          uint32(device.BCDDevice),
				DeviceClass:        uint32(device.DeviceClass),
				DeviceSubClass:     uint32(device.DeviceSubClass),
				DeviceProtocol:     uint32(device.DeviceProtocol),
				ConfigurationValue: uint32(device.ConfigurationValue),
				NumConfigurations:  uint32(device.NumConfigurations),
				Interfaces:         interfaces,
				Serial:             device.Serial,
				Product:            device.Product,
			},
			BusId:    device.BusID,
			StableId: device.StableID,
			Backend:  USBBackend(device.Backend),
			State:    USBDeviceState(device.State),
		})
	}
	return out
}

func addUSBDevice(ctx context.Context, serviceManager adapter.ServiceManager, send func(*USBServerMessage) error, attach *USBDeviceAttach) (*usbProvidedDevice, error) {
	serverService, found := serviceManager.Get(attach.GetServerTag())
	if !found {
		return nil, E.New("usbip-server not found: ", attach.GetServerTag())
	}
	provider, isDynamic := serverService.(adapter.USBIPDynamicServer)
	if !isDynamic {
		return nil, E.New("service ", attach.GetServerTag(), " is not a dynamic usbip-server")
	}
	descriptor := attach.GetDescriptor_()
	if descriptor == nil {
		return nil, E.New("missing device descriptor")
	}
	device := &usbProvidedDevice{
		deviceID: descriptor.GetDeviceId(),
		provider: provider,
		send:     send,
		ctx:      ctx,
		pending:  make(map[uint64]chan *USBURBResponse),
	}
	entry := usbDeviceEntryFromDescriptor(descriptor)
	busID, err := provider.AddDevice(usbip.ProvidedDeviceInfo{Entry: entry}, device)
	if err != nil {
		return nil, err
	}
	device.busID = busID
	return device, nil
}

func usbDeviceEntryFromDescriptor(descriptor *USBDeviceDescriptor) usbip.DeviceEntry {
	deviceID := descriptor.GetDeviceId()
	interfaces := usbInterfacesFromProto(descriptor.GetInterfaces())
	info := usbip.DeviceInfoTruncated{
		BusNum:              descriptor.GetBusNum(),
		DevNum:              descriptor.GetDevNum(),
		Speed:               descriptor.GetSpeed(),
		IDVendor:            uint16(descriptor.GetVendorId()),
		IDProduct:           uint16(descriptor.GetProductId()),
		BCDDevice:           uint16(descriptor.GetBcdDevice()),
		BDeviceClass:        uint8(descriptor.GetDeviceClass()),
		BDeviceSubClass:     uint8(descriptor.GetDeviceSubClass()),
		BDeviceProtocol:     uint8(descriptor.GetDeviceProtocol()),
		BConfigurationValue: uint8(descriptor.GetConfigurationValue()),
		BNumConfigurations:  uint8(descriptor.GetNumConfigurations()),
		BNumInterfaces:      uint8(len(interfaces)),
	}
	copy(info.BusID[:], deviceID)
	return usbip.DeviceEntry{
		Info:       info,
		Interfaces: interfaces,
		Serial:     descriptor.GetSerial(),
		Product:    descriptor.GetProduct(),
	}
}

// sing-usbip calls Submit concurrently across endpoints for a single device.
type usbProvidedDevice struct {
	deviceID string
	busID    string
	provider adapter.USBIPDynamicServer
	send     func(*USBServerMessage) error
	ctx      context.Context

	seq     atomic.Uint64
	access  sync.Mutex
	pending map[uint64]chan *USBURBResponse
	closed  bool
}

func (d *usbProvidedDevice) Submit(request usbip.URBRequest) usbip.URBResponse {
	seq := d.seq.Add(1)
	responseChan := make(chan *USBURBResponse, 1)
	d.access.Lock()
	if d.closed {
		d.access.Unlock()
		return usbip.URBResponse{Error: E.New("device detached")}
	}
	d.pending[seq] = responseChan
	d.access.Unlock()
	defer func() {
		d.access.Lock()
		delete(d.pending, seq)
		d.access.Unlock()
	}()

	directionIn := request.Endpoint&0x80 != 0
	message := &USBURBRequest{
		DeviceId:             d.deviceID,
		Seq:                  seq,
		Endpoint:             uint32(request.Endpoint),
		DirectionIn:          directionIn,
		TransferFlags:        uint32(request.Command.TransferFlags),
		Setup:                append([]byte(nil), request.Command.Setup[:]...),
		TransferBufferLength: uint32(request.Command.TransferBufferLength),
		NumberOfPackets:      request.Command.NumberOfPackets,
		StartFrame:           request.Command.StartFrame,
		Interval:             request.Command.Interval,
		IsoPackets:           isoPacketsToProto(request.IsoPackets),
	}
	if !directionIn {
		message.OutData = request.Buffer
	}
	sendErr := d.send(&USBServerMessage{Message: &USBServerMessage_UrbRequest{UrbRequest: message}})
	if sendErr != nil {
		return usbip.URBResponse{Error: sendErr}
	}
	select {
	case <-d.ctx.Done():
		return usbip.URBResponse{Error: d.ctx.Err()}
	case response := <-responseChan:
		result := usbip.URBResponse{
			Status:       response.GetStatus(),
			ActualLength: response.GetActualLength(),
			IsoPackets:   isoPacketsFromProto(response.GetIsoPackets()),
		}
		if directionIn {
			result.Buffer = response.GetInData()
		}
		return result
	}
}

func (d *usbProvidedDevice) AbortEndpoint(endpoint uint8) error {
	return d.send(&USBServerMessage{Message: &USBServerMessage_Abort{Abort: &USBEndpointAbort{
		DeviceId: d.deviceID,
		Endpoint: uint32(endpoint),
	}}})
}

func (d *usbProvidedDevice) deliver(response *USBURBResponse) {
	d.access.Lock()
	responseChan, found := d.pending[response.GetSeq()]
	d.access.Unlock()
	if !found {
		return
	}
	select {
	case responseChan <- response:
	default:
	}
}

func (d *usbProvidedDevice) close() {
	d.access.Lock()
	if d.closed {
		d.access.Unlock()
		return
	}
	d.closed = true
	d.access.Unlock()
	if d.busID != "" {
		d.provider.RemoveDevice(d.busID)
	}
}

func usbInterfacesFromProto(interfaces []*USBInterface) []usbip.DeviceInterface {
	if len(interfaces) == 0 {
		return nil
	}
	deviceInterfaces := make([]usbip.DeviceInterface, 0, len(interfaces))
	for _, deviceInterface := range interfaces {
		deviceInterfaces = append(deviceInterfaces, usbip.DeviceInterface{
			BInterfaceClass:    uint8(deviceInterface.GetInterfaceClass()),
			BInterfaceSubClass: uint8(deviceInterface.GetInterfaceSubClass()),
			BInterfaceProtocol: uint8(deviceInterface.GetInterfaceProtocol()),
		})
	}
	return deviceInterfaces
}

func isoPacketsToProto(packets []usbip.IsoPacketDescriptor) []*USBIsoPacket {
	if len(packets) == 0 {
		return nil
	}
	out := make([]*USBIsoPacket, 0, len(packets))
	for _, packet := range packets {
		out = append(out, &USBIsoPacket{
			Offset:       packet.Offset,
			Length:       packet.Length,
			ActualLength: packet.ActualLength,
			Status:       packet.Status,
		})
	}
	return out
}

func isoPacketsFromProto(packets []*USBIsoPacket) []usbip.IsoPacketDescriptor {
	if len(packets) == 0 {
		return nil
	}
	out := make([]usbip.IsoPacketDescriptor, 0, len(packets))
	for _, packet := range packets {
		out = append(out, usbip.IsoPacketDescriptor{
			Offset:       packet.GetOffset(),
			Length:       packet.GetLength(),
			ActualLength: packet.GetActualLength(),
			Status:       packet.GetStatus(),
		})
	}
	return out
}
