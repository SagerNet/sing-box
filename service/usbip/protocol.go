//go:build linux || (darwin && cgo) || windows

package usbip

import (
	"encoding/binary"
	"io"
	"strings"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"
)

const (
	DefaultPort = 3240

	ProtocolVersion uint16 = 0x0111

	OpReqDevList uint16 = 0x8005
	OpRepDevList uint16 = 0x0005
	OpReqImport  uint16 = 0x8003
	OpRepImport  uint16 = 0x0003

	OpStatusOK    uint32 = 0
	OpStatusError uint32 = 1

	maxOpRepDevListEntries   = 4096
	maxOpRepDevListBodyBytes = 8 << 20
	deviceInfoWireSize       = 312
	deviceInterfaceWireSize  = 4
)

// DeviceInfoTruncated and DeviceInterface are serialized field-for-field
// by binary.Write with no padding, so their in-memory size equals their
// wire size. These two-sided constant assertions pin the hand-coded wire
// bounds to the structs: a field added, removed, or resized changes the
// kernel-visible layout and fails the build here instead of silently
// mis-bounding the reader, whose only other safety net is the privileged
// interop suite.
const (
	_ = uint(unsafe.Sizeof(DeviceInfoTruncated{})) - deviceInfoWireSize
	_ = deviceInfoWireSize - uint(unsafe.Sizeof(DeviceInfoTruncated{}))
	_ = uint(unsafe.Sizeof(DeviceInterface{})) - deviceInterfaceWireSize
	_ = deviceInterfaceWireSize - uint(unsafe.Sizeof(DeviceInterface{}))
)

const (
	SpeedUnknown   uint32 = 0
	SpeedLow       uint32 = 1
	SpeedFull      uint32 = 2
	SpeedHigh      uint32 = 3
	SpeedWireless  uint32 = 4
	SpeedSuper     uint32 = 5
	SpeedSuperPlus uint32 = 6
)

type OpHeader struct {
	Version uint16
	Code    uint16
	Status  uint32
}

type DeviceInfoTruncated struct {
	Path                [256]byte
	BusID               [32]byte
	BusNum              uint32
	DevNum              uint32
	Speed               uint32
	IDVendor            uint16
	IDProduct           uint16
	BCDDevice           uint16
	BDeviceClass        uint8
	BDeviceSubClass     uint8
	BDeviceProtocol     uint8
	BConfigurationValue uint8
	BNumConfigurations  uint8
	BNumInterfaces      uint8
}

type DeviceInterface struct {
	BInterfaceClass    uint8
	BInterfaceSubClass uint8
	BInterfaceProtocol uint8
	Padding            uint8
}

type DeviceEntry struct {
	Info       DeviceInfoTruncated
	Interfaces []DeviceInterface
	Serial     string
}

func WriteOpHeader(w io.Writer, code uint16, status uint32) error {
	return binary.Write(w, binary.BigEndian, OpHeader{
		Version: ProtocolVersion,
		Code:    code,
		Status:  status,
	})
}

func ReadOpHeader(r io.Reader) (OpHeader, error) {
	var h OpHeader
	err := binary.Read(r, binary.BigEndian, &h)
	if err != nil {
		return h, err
	}
	return h, nil
}

func ParseOpHeader(raw []byte) OpHeader {
	return OpHeader{
		Version: binary.BigEndian.Uint16(raw[:2]),
		Code:    binary.BigEndian.Uint16(raw[2:4]),
		Status:  binary.BigEndian.Uint32(raw[4:8]),
	}
}

func WriteOpReqImport(w io.Writer, busid string) error {
	err := WriteOpHeader(w, OpReqImport, OpStatusOK)
	if err != nil {
		return err
	}
	var field [32]byte
	if len(busid) >= len(field) {
		return E.New("busid too long: ", busid)
	}
	copy(field[:], busid)
	return binary.Write(w, binary.BigEndian, field)
}

func ReadOpReqImportBody(r io.Reader) (string, error) {
	var field [32]byte
	_, err := io.ReadFull(r, field[:])
	if err != nil {
		return "", err
	}
	return cstring(field[:]), nil
}

func WriteOpRepImport(w io.Writer, code uint16, status uint32, info *DeviceInfoTruncated) error {
	err := WriteOpHeader(w, code, status)
	if err != nil {
		return err
	}
	if status != OpStatusOK {
		return nil
	}
	if info == nil {
		return E.New("OP_REP_IMPORT success without device info")
	}
	return binary.Write(w, binary.BigEndian, info)
}

func ReadOpRepImportBody(r io.Reader) (DeviceInfoTruncated, error) {
	var info DeviceInfoTruncated
	err := binary.Read(r, binary.BigEndian, &info)
	if err != nil {
		return info, err
	}
	return info, nil
}

func WriteOpRepDevList(w io.Writer, entries []DeviceEntry) error {
	err := WriteOpHeader(w, OpRepDevList, OpStatusOK)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, uint32(len(entries)))
	if err != nil {
		return err
	}
	for i := range entries {
		err = binary.Write(w, binary.BigEndian, &entries[i].Info)
		if err != nil {
			return err
		}
		for j := range entries[i].Interfaces {
			err = binary.Write(w, binary.BigEndian, &entries[i].Interfaces[j])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func ReadOpRepDevListBody(r io.Reader) ([]DeviceEntry, error) {
	var count uint32
	err := binary.Read(r, binary.BigEndian, &count)
	if err != nil {
		return nil, err
	}
	if count > maxOpRepDevListEntries {
		return nil, E.New("OP_REP_DEVLIST device count too large: ", count)
	}
	bodyBytes := uint64(4)
	entries := make([]DeviceEntry, int(count))
	for i := range entries {
		err = binary.Read(r, binary.BigEndian, &entries[i].Info)
		if err != nil {
			return nil, err
		}
		entries[i].Serial = entries[i].Info.SerialString()
		bodyBytes += deviceInfoWireSize
		if bodyBytes > maxOpRepDevListBodyBytes {
			return nil, E.New("OP_REP_DEVLIST body too large")
		}
		n := int(entries[i].Info.BNumInterfaces)
		if n > 0 {
			bodyBytes += uint64(n) * deviceInterfaceWireSize
			if bodyBytes > maxOpRepDevListBodyBytes {
				return nil, E.New("OP_REP_DEVLIST interface data too large")
			}
			entries[i].Interfaces = make([]DeviceInterface, n)
			for j := range entries[i].Interfaces {
				err = binary.Read(r, binary.BigEndian, &entries[i].Interfaces[j])
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return entries, nil
}

func (d *DeviceInfoTruncated) BusIDString() string {
	return cstring(d.BusID[:])
}

func (d *DeviceInfoTruncated) SerialString() string {
	meta := trailingCString(d.Path[:])
	serial, ok := strings.CutPrefix(meta, "serial=")
	if !ok {
		return ""
	}
	return serial
}

func (d *DeviceInfoTruncated) DevID() uint32 {
	return (d.BusNum << 16) | (d.DevNum & 0xffff)
}

func encodePathField(dst *[256]byte, path string, serial string) {
	*dst = [256]byte{}
	pathLen := copy(dst[:len(dst)-1], path)
	if serial == "" {
		return
	}
	trailer := "serial=" + serial
	trailerStart := pathLen + 1
	if trailerStart+len(trailer)+1 > len(dst) {
		return
	}
	copy(dst[trailerStart:], trailer)
}

func cstring(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func trailingCString(b []byte) string {
	for i, c := range b {
		if c != 0 {
			continue
		}
		if i+1 >= len(b) {
			return ""
		}
		return cstring(b[i+1:])
	}
	return ""
}

func entryDeviceKey(entry DeviceEntry) DeviceKey {
	serial := entry.Serial
	if serial == "" {
		serial = entry.Info.SerialString()
	}
	return DeviceKey{
		BusID:     entry.Info.BusIDString(),
		VendorID:  entry.Info.IDVendor,
		ProductID: entry.Info.IDProduct,
		Serial:    serial,
	}
}
