package usbip

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"reflect"
	"sort"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

const (
	controlProtocolVersion uint8 = 1

	controlFrameHello          uint8 = 1
	controlFrameAck            uint8 = 2
	controlFrameChanged        uint8 = 3
	controlFramePing           uint8 = 4
	controlFramePong           uint8 = 5
	controlFrameDeviceSnapshot uint8 = 6
	controlFrameDeviceDelta    uint8 = 7
	controlFrameLeaseRequest   uint8 = 8
	controlFrameLeaseResponse  uint8 = 9

	controlCapabilityChanged       uint32 = 1 << 0
	controlCapabilityPingPong      uint32 = 1 << 1
	controlCapabilityPayloadFrames uint32 = 1 << 2
	controlCapabilityDeviceStateV2 uint32 = 1 << 3
	controlCapabilityImportLease   uint32 = 1 << 4
	controlRequiredCapabilities           = controlCapabilityChanged | controlCapabilityPingPong
	controlExtensionCapabilities          = controlCapabilityPayloadFrames | controlCapabilityDeviceStateV2 | controlCapabilityImportLease
	controlCapabilities                   = controlRequiredCapabilities | controlExtensionCapabilities

	controlPrefaceSize      = 8
	controlFrameSize        = 16
	maxControlPayloadLength = 64<<10 - 1

	deviceStateAvailable   = "available"
	deviceStateBusy        = "busy"
	deviceStateUnavailable = "unavailable"

	importLeaseTTL = 10 * time.Second
)

var controlPreface = [controlPrefaceSize]byte{'S', 'B', 'U', 'S', 'B', 'I', 'P', '1'}

type controlFrame struct {
	Type          uint8
	Version       uint8
	PayloadLength uint16
	Capabilities  uint32
	Sequence      uint64
}

type controlMessage struct {
	Frame   controlFrame
	Payload []byte
}

type controlOutboundMessage struct {
	Frame   controlFrame
	Payload []byte
}

type DeviceInterfaceV2 struct {
	Class    uint8 `json:"class"`
	SubClass uint8 `json:"subclass"`
	Protocol uint8 `json:"protocol"`
}

type DeviceInfoV2 struct {
	BusID              string              `json:"busid"`
	StableID           string              `json:"stable_id,omitempty"`
	Backend            string              `json:"backend,omitempty"`
	Path               string              `json:"path,omitempty"`
	Serial             string              `json:"serial,omitempty"`
	VendorID           uint16              `json:"vendor_id"`
	ProductID          uint16              `json:"product_id"`
	BCDDevice          uint16              `json:"bcd_device,omitempty"`
	Speed              uint32              `json:"speed"`
	DeviceClass        uint8               `json:"device_class"`
	DeviceSubClass     uint8               `json:"device_subclass"`
	DeviceProtocol     uint8               `json:"device_protocol"`
	ConfigurationValue uint8               `json:"configuration_value"`
	NumConfigurations  uint8               `json:"num_configurations"`
	NumInterfaces      uint8               `json:"num_interfaces"`
	Interfaces         []DeviceInterfaceV2 `json:"interfaces,omitempty"`
	State              string              `json:"state"`
	StatusCode         int                 `json:"status_code,omitempty"`
	StatusReason       string              `json:"status_reason,omitempty"`
}

type controlDeviceSnapshot struct {
	Sequence uint64         `json:"sequence"`
	Devices  []DeviceInfoV2 `json:"devices"`
}

type controlDeviceDelta struct {
	Sequence uint64         `json:"sequence"`
	Added    []DeviceInfoV2 `json:"added,omitempty"`
	Updated  []DeviceInfoV2 `json:"updated,omitempty"`
	Removed  []string       `json:"removed,omitempty"`
}

type controlLeaseRequest struct {
	BusID       string `json:"busid"`
	ClientNonce uint64 `json:"client_nonce"`
}

type controlLeaseResponse struct {
	BusID        string `json:"busid"`
	LeaseID      uint64 `json:"lease_id,omitempty"`
	ClientNonce  uint64 `json:"client_nonce"`
	Generation   uint64 `json:"generation,omitempty"`
	TTLMillis    int64  `json:"ttl_millis,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type serverImportLease struct {
	ID           uint64
	SubscriberID uint64
	BusID        string
	ClientNonce  uint64
	Generation   uint64
	Expires      time.Time
}

func WriteControlPreface(w io.Writer) error {
	_, err := w.Write(controlPreface[:])
	return err
}

func IsControlPreface(raw []byte) bool {
	if len(raw) != len(controlPreface) {
		return false
	}
	for i := range controlPreface {
		if raw[i] != controlPreface[i] {
			return false
		}
	}
	return true
}

func WriteControlHello(w io.Writer) error {
	return writeControlFrame(w, controlFrame{
		Type:         controlFrameHello,
		Version:      controlProtocolVersion,
		Capabilities: controlCapabilities,
	})
}

func WriteControlAck(w io.Writer, sequence uint64) error {
	return writeControlAckWithCapabilities(w, sequence, controlCapabilities)
}

func writeControlAckWithCapabilities(w io.Writer, sequence uint64, capabilities uint32) error {
	return writeControlFrame(w, controlFrame{
		Type:         controlFrameAck,
		Version:      controlProtocolVersion,
		Capabilities: capabilities,
		Sequence:     sequence,
	})
}

func WriteControlChanged(w io.Writer, sequence uint64) error {
	return writeControlFrame(w, controlFrame{
		Type:     controlFrameChanged,
		Version:  controlProtocolVersion,
		Sequence: sequence,
	})
}

func WriteControlPing(w io.Writer) error {
	return writeControlFrame(w, controlFrame{
		Type:    controlFramePing,
		Version: controlProtocolVersion,
	})
}

func WriteControlPong(w io.Writer) error {
	return writeControlFrame(w, controlFrame{
		Type:    controlFramePong,
		Version: controlProtocolVersion,
	})
}

func ReadControlFrame(r io.Reader) (controlFrame, error) {
	message, err := readControlMessage(r)
	if err != nil {
		return controlFrame{}, err
	}
	if len(message.Payload) > 0 {
		return controlFrame{}, E.New("unexpected control payload length ", len(message.Payload))
	}
	return message.Frame, nil
}

func readControlMessage(r io.Reader) (controlMessage, error) {
	var raw [controlFrameSize]byte
	if _, err := io.ReadFull(r, raw[:]); err != nil {
		return controlMessage{}, err
	}
	frame := controlFrame{
		Type:          raw[0],
		Version:       raw[1],
		PayloadLength: binary.BigEndian.Uint16(raw[2:4]),
		Capabilities:  binary.BigEndian.Uint32(raw[4:8]),
		Sequence:      binary.BigEndian.Uint64(raw[8:16]),
	}
	var payload []byte
	if frame.PayloadLength > 0 {
		payload = make([]byte, frame.PayloadLength)
		if _, err := io.ReadFull(r, payload); err != nil {
			return controlMessage{}, err
		}
	}
	return controlMessage{Frame: frame, Payload: payload}, nil
}

func writeControlFrame(w io.Writer, frame controlFrame) error {
	return writeControlMessage(w, frame, nil)
}

func writeControlMessage(w io.Writer, frame controlFrame, payload any) error {
	rawPayload, err := marshalControlPayload(payload)
	if err != nil {
		return err
	}
	if len(rawPayload) > maxControlPayloadLength {
		return E.New("control payload too large: ", len(rawPayload))
	}
	frame.PayloadLength = uint16(len(rawPayload))
	var raw [controlFrameSize]byte
	raw[0] = frame.Type
	raw[1] = frame.Version
	binary.BigEndian.PutUint16(raw[2:4], frame.PayloadLength)
	binary.BigEndian.PutUint32(raw[4:8], frame.Capabilities)
	binary.BigEndian.PutUint64(raw[8:16], frame.Sequence)
	if _, err := w.Write(raw[:]); err != nil {
		return err
	}
	if len(rawPayload) == 0 {
		return nil
	}
	_, err = w.Write(rawPayload)
	return err
}

func marshalControlPayload(payload any) ([]byte, error) {
	switch value := payload.(type) {
	case nil:
		return nil, nil
	case []byte:
		return value, nil
	default:
		return json.Marshal(value)
	}
}

func unmarshalControlPayload(payload []byte, value any) error {
	if len(payload) == 0 {
		return E.New("missing control payload")
	}
	return json.Unmarshal(payload, value)
}

func negotiatedControlCapabilities(peer uint32) uint32 {
	return peer & controlCapabilities
}

func supportsControlExtensions(capabilities uint32) bool {
	return capabilities&controlExtensionCapabilities == controlExtensionCapabilities
}

func deviceInfoV2FromEntry(entry DeviceEntry, backend string, stableID string, state string, statusCode int, statusReason string) DeviceInfoV2 {
	interfaces := make([]DeviceInterfaceV2, len(entry.Interfaces))
	for i := range entry.Interfaces {
		interfaces[i] = DeviceInterfaceV2{
			Class:    entry.Interfaces[i].BInterfaceClass,
			SubClass: entry.Interfaces[i].BInterfaceSubClass,
			Protocol: entry.Interfaces[i].BInterfaceProtocol,
		}
	}
	if state == "" {
		state = deviceStateAvailable
	}
	return DeviceInfoV2{
		BusID:              entry.Info.BusIDString(),
		StableID:           stableID,
		Backend:            backend,
		Path:               entry.Info.PathString(),
		Serial:             entrySerial(entry),
		VendorID:           entry.Info.IDVendor,
		ProductID:          entry.Info.IDProduct,
		BCDDevice:          entry.Info.BCDDevice,
		Speed:              entry.Info.Speed,
		DeviceClass:        entry.Info.BDeviceClass,
		DeviceSubClass:     entry.Info.BDeviceSubClass,
		DeviceProtocol:     entry.Info.BDeviceProtocol,
		ConfigurationValue: entry.Info.BConfigurationValue,
		NumConfigurations:  entry.Info.BNumConfigurations,
		NumInterfaces:      entry.Info.BNumInterfaces,
		Interfaces:         interfaces,
		State:              state,
		StatusCode:         statusCode,
		StatusReason:       statusReason,
	}
}

func (d DeviceInfoV2) toDeviceEntry() DeviceEntry {
	var info DeviceInfoTruncated
	encodePathField(&info.Path, d.Path)
	copy(info.BusID[:], d.BusID)
	info.Speed = d.Speed
	info.IDVendor = d.VendorID
	info.IDProduct = d.ProductID
	info.BCDDevice = d.BCDDevice
	info.BDeviceClass = d.DeviceClass
	info.BDeviceSubClass = d.DeviceSubClass
	info.BDeviceProtocol = d.DeviceProtocol
	info.BConfigurationValue = d.ConfigurationValue
	info.BNumConfigurations = d.NumConfigurations
	info.BNumInterfaces = d.NumInterfaces
	interfaces := make([]DeviceInterface, len(d.Interfaces))
	for i := range d.Interfaces {
		interfaces[i] = DeviceInterface{
			BInterfaceClass:    d.Interfaces[i].Class,
			BInterfaceSubClass: d.Interfaces[i].SubClass,
			BInterfaceProtocol: d.Interfaces[i].Protocol,
		}
	}
	return DeviceEntry{Info: info, Interfaces: interfaces, Serial: d.Serial}
}

func (d DeviceInfoV2) key() DeviceKey {
	return DeviceKey{
		BusID:     d.BusID,
		VendorID:  d.VendorID,
		ProductID: d.ProductID,
		Serial:    d.Serial,
	}
}

func (d DeviceInfoV2) available() bool {
	return d.State == "" || d.State == deviceStateAvailable
}

func deviceInfoV2Map(devices []DeviceInfoV2) map[string]DeviceInfoV2 {
	out := make(map[string]DeviceInfoV2, len(devices))
	for _, device := range devices {
		if device.BusID == "" {
			continue
		}
		out[device.BusID] = device
	}
	return out
}

func sortedDeviceInfoV2Values(devices map[string]DeviceInfoV2) []DeviceInfoV2 {
	busids := make([]string, 0, len(devices))
	for busid := range devices {
		busids = append(busids, busid)
	}
	sort.Strings(busids)
	out := make([]DeviceInfoV2, 0, len(busids))
	for _, busid := range busids {
		out = append(out, devices[busid])
	}
	return out
}

func deviceInfoV2ToEntries(devices []DeviceInfoV2, availableOnly bool) []DeviceEntry {
	entries := make([]DeviceEntry, 0, len(devices))
	for _, device := range devices {
		if availableOnly && !device.available() {
			continue
		}
		entries = append(entries, device.toDeviceEntry())
	}
	return entries
}

func buildControlDeviceDelta(sequence uint64, previous map[string]DeviceInfoV2, current map[string]DeviceInfoV2) controlDeviceDelta {
	delta := controlDeviceDelta{Sequence: sequence}
	for busid, device := range current {
		prev, ok := previous[busid]
		if !ok {
			delta.Added = append(delta.Added, device)
			continue
		}
		if !reflect.DeepEqual(prev, device) {
			delta.Updated = append(delta.Updated, device)
		}
	}
	for busid := range previous {
		if _, ok := current[busid]; !ok {
			delta.Removed = append(delta.Removed, busid)
		}
	}
	sort.Slice(delta.Added, func(i, j int) bool { return delta.Added[i].BusID < delta.Added[j].BusID })
	sort.Slice(delta.Updated, func(i, j int) bool { return delta.Updated[i].BusID < delta.Updated[j].BusID })
	sort.Strings(delta.Removed)
	return delta
}
