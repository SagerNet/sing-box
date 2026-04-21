package usbip

import (
	"encoding/binary"
	"io"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
)

// Wire constants.
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

// USB speeds (enum usb_device_speed).
const (
	SpeedUnknown   uint32 = 0
	SpeedLow       uint32 = 1
	SpeedFull      uint32 = 2
	SpeedHigh      uint32 = 3
	SpeedWireless  uint32 = 4
	SpeedSuper     uint32 = 5
	SpeedSuperPlus uint32 = 6
)

// OpHeader is the 8-byte header prefix of every OP message.
type OpHeader struct {
	Version uint16
	Code    uint16
	Status  uint32
}

// DeviceInfoTruncated is the 312-byte device descriptor shared by OP_REP_DEVLIST
// entries and OP_REP_IMPORT bodies.
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

// DeviceInterface is the 4-byte per-interface descriptor carried in OP_REP_DEVLIST.
type DeviceInterface struct {
	BInterfaceClass    uint8
	BInterfaceSubClass uint8
	BInterfaceProtocol uint8
	Padding            uint8
}

// DeviceEntry is one element of an OP_REP_DEVLIST body.
type DeviceEntry struct {
	Info       DeviceInfoTruncated
	Interfaces []DeviceInterface
}

// WriteOpHeader emits the 8-byte OP header.
func WriteOpHeader(w io.Writer, code uint16, status uint32) error {
	return binary.Write(w, binary.BigEndian, OpHeader{
		Version: ProtocolVersion,
		Code:    code,
		Status:  status,
	})
}

// ReadOpHeader consumes the 8-byte OP header and returns it.
func ReadOpHeader(r io.Reader) (OpHeader, error) {
	var h OpHeader
	if err := binary.Read(r, binary.BigEndian, &h); err != nil {
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

// WriteOpReqImport sends OP_REQ_IMPORT for busid (8 + 32 = 40 bytes).
func WriteOpReqImport(w io.Writer, busid string) error {
	if err := WriteOpHeader(w, OpReqImport, OpStatusOK); err != nil {
		return err
	}
	var field [32]byte
	if len(busid) >= len(field) {
		return E.New("busid too long: ", busid)
	}
	copy(field[:], busid)
	return binary.Write(w, binary.BigEndian, field)
}

// ReadOpReqImportBody reads the 32-byte busid that follows the OP header.
func ReadOpReqImportBody(r io.Reader) (string, error) {
	var field [32]byte
	if _, err := io.ReadFull(r, field[:]); err != nil {
		return "", err
	}
	return cstring(field[:]), nil
}

// WriteOpRepImport sends OP_REP_IMPORT. If status != OpStatusOK, info is omitted.
func WriteOpRepImport(w io.Writer, status uint32, info *DeviceInfoTruncated) error {
	if err := WriteOpHeader(w, OpRepImport, status); err != nil {
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

// ReadOpRepImportBody reads the 312-byte device info that follows a successful header.
func ReadOpRepImportBody(r io.Reader) (DeviceInfoTruncated, error) {
	var info DeviceInfoTruncated
	if err := binary.Read(r, binary.BigEndian, &info); err != nil {
		return info, err
	}
	return info, nil
}

// WriteOpRepDevList emits an OP_REP_DEVLIST response with the given entries.
func WriteOpRepDevList(w io.Writer, entries []DeviceEntry) error {
	if err := WriteOpHeader(w, OpRepDevList, OpStatusOK); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(entries))); err != nil {
		return err
	}
	for i := range entries {
		if err := binary.Write(w, binary.BigEndian, &entries[i].Info); err != nil {
			return err
		}
		for j := range entries[i].Interfaces {
			if err := binary.Write(w, binary.BigEndian, &entries[i].Interfaces[j]); err != nil {
				return err
			}
		}
	}
	return nil
}

// ReadOpRepDevListBody reads what follows a devlist OP header: a uint32 count
// plus that many device entries with their per-interface tails.
func ReadOpRepDevListBody(r io.Reader) ([]DeviceEntry, error) {
	var count uint32
	if err := binary.Read(r, binary.BigEndian, &count); err != nil {
		return nil, err
	}
	if count > maxOpRepDevListEntries {
		return nil, E.New("OP_REP_DEVLIST device count too large: ", count)
	}
	bodyBytes := uint64(4)
	entries := make([]DeviceEntry, int(count))
	for i := range entries {
		if err := binary.Read(r, binary.BigEndian, &entries[i].Info); err != nil {
			return nil, err
		}
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
				if err := binary.Read(r, binary.BigEndian, &entries[i].Interfaces[j]); err != nil {
					return nil, err
				}
			}
		}
	}
	return entries, nil
}

// BusID extracts the null-terminated busid from a DeviceInfoTruncated.
func (d *DeviceInfoTruncated) BusIDString() string {
	return cstring(d.BusID[:])
}

// PathString extracts the null-terminated sysfs path.
func (d *DeviceInfoTruncated) PathString() string {
	return cstring(d.Path[:])
}

func (d *DeviceInfoTruncated) SerialString() string {
	meta := trailingCString(d.Path[:])
	serial, ok := strings.CutPrefix(meta, "serial=")
	if !ok {
		return ""
	}
	return serial
}

// DevID packs busnum/devnum the way vhci_hcd.attach expects.
func (d *DeviceInfoTruncated) DevID() uint32 {
	return (d.BusNum << 16) | (d.DevNum & 0xffff)
}

func encodePathField(dst *[256]byte, path, serial string) {
	copy(dst[:], path)
	if serial == "" || len(path) >= len(dst)-1 {
		return
	}
	copy(dst[len(path)+1:], "serial="+serial)
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
