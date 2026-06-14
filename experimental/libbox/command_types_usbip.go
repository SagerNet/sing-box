package libbox

import (
	"context"
	"os"
	"sync"

	"github.com/sagernet/sing-box/daemon"
)

type USBProviderHandler interface {
	OnReady(deviceID string, busID string)
	OnURBRequest(request *USBURBRequest)
	OnAbort(deviceID string, endpoint int32)
	OnError(deviceID string, message string)
}

type USBIsoPacket struct {
	Offset       int32
	Length       int32
	ActualLength int32
	Status       int32
}

type USBDeviceDescriptor struct {
	ServerTag          string
	DeviceID           string
	BusNum             int32
	DevNum             int32
	Speed              int32
	VendorID           int32
	ProductID          int32
	BCDDevice          int32
	DeviceClass        int32
	DeviceSubClass     int32
	DeviceProtocol     int32
	ConfigurationValue int32
	NumConfigurations  int32
	Serial             string
	Product            string

	interfaces []*daemon.USBInterface
}

func NewUSBDeviceDescriptor(serverTag string, deviceID string) *USBDeviceDescriptor {
	return &USBDeviceDescriptor{ServerTag: serverTag, DeviceID: deviceID}
}

func (d *USBDeviceDescriptor) AddInterface(interfaceClass int32, interfaceSubClass int32, interfaceProtocol int32) {
	d.interfaces = append(d.interfaces, &daemon.USBInterface{
		InterfaceClass:    uint32(interfaceClass),
		InterfaceSubClass: uint32(interfaceSubClass),
		InterfaceProtocol: uint32(interfaceProtocol),
	})
}

func (d *USBDeviceDescriptor) toProto() *daemon.USBDeviceAttach {
	return &daemon.USBDeviceAttach{
		ServerTag: d.ServerTag,
		Descriptor_: &daemon.USBDeviceDescriptor{
			DeviceId:           d.DeviceID,
			BusNum:             uint32(d.BusNum),
			DevNum:             uint32(d.DevNum),
			Speed:              uint32(d.Speed),
			VendorId:           uint32(d.VendorID),
			ProductId:          uint32(d.ProductID),
			BcdDevice:          uint32(d.BCDDevice),
			DeviceClass:        uint32(d.DeviceClass),
			DeviceSubClass:     uint32(d.DeviceSubClass),
			DeviceProtocol:     uint32(d.DeviceProtocol),
			ConfigurationValue: uint32(d.ConfigurationValue),
			NumConfigurations:  uint32(d.NumConfigurations),
			Interfaces:         d.interfaces,
			Serial:             d.Serial,
			Product:            d.Product,
		},
	}
}

type USBURBRequest struct {
	DeviceID             string
	Seq                  int64
	Endpoint             int32
	DirectionIn          bool
	TransferFlags        int32
	Setup                []byte
	TransferBufferLength int32
	OutData              []byte
	NumberOfPackets      int32
	StartFrame           int32
	Interval             int32

	isoPackets []*daemon.USBIsoPacket
}

func (r *USBURBRequest) IsoPacketCount() int32 {
	return int32(len(r.isoPackets))
}

func (r *USBURBRequest) GetIsoPacket(index int32) *USBIsoPacket {
	if index < 0 || int(index) >= len(r.isoPackets) {
		return nil
	}
	packet := r.isoPackets[index]
	return &USBIsoPacket{
		Offset:       packet.GetOffset(),
		Length:       packet.GetLength(),
		ActualLength: packet.GetActualLength(),
		Status:       packet.GetStatus(),
	}
}

func usbURBRequestFromGRPC(request *daemon.USBURBRequest) *USBURBRequest {
	return &USBURBRequest{
		DeviceID:             request.GetDeviceId(),
		Seq:                  int64(request.GetSeq()),
		Endpoint:             int32(request.GetEndpoint()),
		DirectionIn:          request.GetDirectionIn(),
		TransferFlags:        int32(request.GetTransferFlags()),
		Setup:                request.GetSetup(),
		TransferBufferLength: int32(request.GetTransferBufferLength()),
		OutData:              request.GetOutData(),
		NumberOfPackets:      request.GetNumberOfPackets(),
		StartFrame:           request.GetStartFrame(),
		Interval:             request.GetInterval(),
		isoPackets:           request.GetIsoPackets(),
	}
}

type USBURBResponse struct {
	DeviceID     string
	Seq          int64
	Status       int32
	ActualLength int32
	InData       []byte

	isoPackets []*daemon.USBIsoPacket
}

func NewUSBURBResponse(deviceID string, seq int64) *USBURBResponse {
	return &USBURBResponse{DeviceID: deviceID, Seq: seq}
}

func (r *USBURBResponse) AddIsoPacket(offset int32, length int32, actualLength int32, status int32) {
	r.isoPackets = append(r.isoPackets, &daemon.USBIsoPacket{
		Offset:       offset,
		Length:       length,
		ActualLength: actualLength,
		Status:       status,
	})
}

func (r *USBURBResponse) toProto() *daemon.USBURBResponse {
	return &daemon.USBURBResponse{
		DeviceId:     r.DeviceID,
		Seq:          uint64(r.Seq),
		Status:       r.Status,
		ActualLength: r.ActualLength,
		InData:       r.InData,
		IsoPackets:   r.isoPackets,
	}
}

type USBProviderSession struct {
	stream     daemon.StartedService_ProvideUSBDevicesClient
	ctx        context.Context
	cancel     context.CancelFunc
	sendAccess sync.Mutex
	closeOnce  sync.Once
	closeDone  chan struct{}
}

func (s *USBProviderSession) send(message *daemon.USBProviderMessage) error {
	s.sendAccess.Lock()
	defer s.sendAccess.Unlock()
	select {
	case <-s.ctx.Done():
		return os.ErrClosed
	default:
	}
	return s.stream.Send(message)
}

func (s *USBProviderSession) AttachDevice(descriptor *USBDeviceDescriptor) error {
	return s.send(&daemon.USBProviderMessage{Message: &daemon.USBProviderMessage_Attach{Attach: descriptor.toProto()}})
}

func (s *USBProviderSession) DetachDevice(deviceID string) error {
	return s.send(&daemon.USBProviderMessage{Message: &daemon.USBProviderMessage_Detach{Detach: &daemon.USBDeviceDetach{DeviceId: deviceID}}})
}

func (s *USBProviderSession) SendURBResponse(response *USBURBResponse) error {
	return s.send(&daemon.USBProviderMessage{Message: &daemon.USBProviderMessage_UrbResponse{UrbResponse: response.toProto()}})
}

func (s *USBProviderSession) Close() error {
	s.closeOnce.Do(func() {
		s.cancel()
		_ = s.stream.CloseSend()
	})
	<-s.closeDone
	return nil
}
