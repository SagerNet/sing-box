package usbip

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
)

func TestControlPrefaceAndFrames(t *testing.T) {
	t.Parallel()

	var preface bytes.Buffer
	require.NoError(t, WriteControlPreface(&preface))
	require.True(t, IsControlPreface(preface.Bytes()))

	corruptedPreface := append([]byte(nil), preface.Bytes()...)
	corruptedPreface[len(corruptedPreface)-1] = '0'
	require.False(t, IsControlPreface(corruptedPreface))
	require.False(t, IsControlPreface(preface.Bytes()[:len(preface.Bytes())-1]))

	testCases := []struct {
		name     string
		write    func(io.Writer) error
		expected controlFrame
	}{
		{
			name:  "hello",
			write: WriteControlHello,
			expected: controlFrame{
				Type:         controlFrameHello,
				Version:      controlProtocolVersion,
				Capabilities: controlCapabilities,
			},
		},
		{
			name:  "ack",
			write: func(writer io.Writer) error { return WriteControlAck(writer, 7) },
			expected: controlFrame{
				Type:         controlFrameAck,
				Version:      controlProtocolVersion,
				Capabilities: controlCapabilities,
				Sequence:     7,
			},
		},
		{
			name:  "changed",
			write: func(writer io.Writer) error { return WriteControlChanged(writer, 9) },
			expected: controlFrame{
				Type:     controlFrameChanged,
				Version:  controlProtocolVersion,
				Sequence: 9,
			},
		},
		{
			name:  "ping",
			write: WriteControlPing,
			expected: controlFrame{
				Type:    controlFramePing,
				Version: controlProtocolVersion,
			},
		},
		{
			name:  "pong",
			write: WriteControlPong,
			expected: controlFrame{
				Type:    controlFramePong,
				Version: controlProtocolVersion,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var buffer bytes.Buffer
			require.NoError(t, testCase.write(&buffer))

			frame, err := ReadControlFrame(&buffer)
			require.NoError(t, err)
			require.Equal(t, testCase.expected, frame)
		})
	}
}

func TestControlMessagePayloadRoundTrip(t *testing.T) {
	t.Parallel()

	payload := controlDeviceSnapshot{
		Sequence: 3,
		Devices: []DeviceInfoV2{{
			BusID:     "1-2",
			StableID:  "usb:1d6b:0002:serial-1",
			Backend:   "linux-sysfs",
			VendorID:  0x1d6b,
			ProductID: 0x0002,
			Speed:     SpeedHigh,
			State:     deviceStateAvailable,
		}},
	}
	var buffer bytes.Buffer
	require.NoError(t, writeControlMessage(&buffer, controlFrame{
		Type:     controlFrameDeviceSnapshot,
		Version:  controlProtocolVersion,
		Sequence: 3,
	}, payload))

	message, err := readControlMessage(&buffer)
	require.NoError(t, err)
	require.Equal(t, controlFrameDeviceSnapshot, message.Frame.Type)
	require.Equal(t, uint64(3), message.Frame.Sequence)
	require.Positive(t, message.Frame.PayloadLength)

	var decoded controlDeviceSnapshot
	require.NoError(t, unmarshalControlPayload(message.Payload, &decoded))
	require.Equal(t, payload, decoded)
}

func TestControlMessagePayloadSizeGuard(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	err := writeControlMessage(&buffer, controlFrame{Type: controlFrameDeviceSnapshot}, bytes.Repeat([]byte{'x'}, maxControlPayloadLength+1))
	require.ErrorContains(t, err, "control payload too large")
}

func TestReadControlFrameRejectsPayload(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	require.NoError(t, writeControlMessage(&buffer, controlFrame{Type: controlFrameDeviceSnapshot}, []byte(`{}`)))

	_, err := ReadControlFrame(&buffer)
	require.ErrorContains(t, err, "unexpected control payload length")
}

func TestOpHeaderRoundTrip(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	require.NoError(t, WriteOpHeader(&buffer, OpReqDevList, OpStatusError))

	header, err := ReadOpHeader(&buffer)
	require.NoError(t, err)
	require.Equal(t, OpHeader{
		Version: ProtocolVersion,
		Code:    OpReqDevList,
		Status:  OpStatusError,
	}, header)

	var raw [8]byte
	binary.BigEndian.PutUint16(raw[:2], ProtocolVersion)
	binary.BigEndian.PutUint16(raw[2:4], OpRepImport)
	binary.BigEndian.PutUint32(raw[4:8], OpStatusOK)
	require.Equal(t, OpHeader{
		Version: ProtocolVersion,
		Code:    OpRepImport,
		Status:  OpStatusOK,
	}, ParseOpHeader(raw[:]))
}

func TestOpReqImportExtRoundTrip(t *testing.T) {
	t.Parallel()

	request := ImportExtRequest{
		BusID:       "1-2",
		LeaseID:     9,
		ClientNonce: 7,
		Flags:       1,
	}
	var buffer bytes.Buffer
	require.NoError(t, WriteOpReqImportExt(&buffer, request))

	header, err := ReadOpHeader(&buffer)
	require.NoError(t, err)
	require.Equal(t, OpReqImportExt, header.Code)
	require.Equal(t, OpStatusOK, header.Status)

	parsed, err := ReadOpReqImportExtBody(&buffer)
	require.NoError(t, err)
	require.Equal(t, request, parsed)
}

func TestOpReqImportRoundTrip(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	require.NoError(t, WriteOpReqImport(&buffer, "1-2"))

	header, err := ReadOpHeader(&buffer)
	require.NoError(t, err)
	require.Equal(t, OpReqImport, header.Code)
	require.Equal(t, OpStatusOK, header.Status)

	busid, err := ReadOpReqImportBody(&buffer)
	require.NoError(t, err)
	require.Equal(t, "1-2", busid)
}

func TestWriteOpReqImportRejectsLongBusID(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	err := WriteOpReqImport(&buffer, strings.Repeat("a", 32))
	require.ErrorContains(t, err, "busid too long")
}

func TestWriteOpRepImportRequiresInfoOnSuccess(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	err := WriteOpRepImport(&buffer, OpStatusOK, nil)
	require.ErrorContains(t, err, "success without device info")
}

func TestOpRepImportRoundTrip(t *testing.T) {
	t.Parallel()

	var info DeviceInfoTruncated
	copy(info.BusID[:], "1-2")
	info.BusNum = 1
	info.DevNum = 2
	info.Speed = SpeedHigh
	info.IDVendor = 0x1d6b
	info.IDProduct = 0x0002

	var buffer bytes.Buffer
	require.NoError(t, WriteOpRepImport(&buffer, OpStatusOK, &info))

	header, err := ReadOpHeader(&buffer)
	require.NoError(t, err)
	require.Equal(t, OpRepImport, header.Code)
	require.Equal(t, OpStatusOK, header.Status)

	body, err := ReadOpRepImportBody(&buffer)
	require.NoError(t, err)
	require.Equal(t, info, body)
}

func TestOpRepDevListRoundTrip(t *testing.T) {
	t.Parallel()

	var path [256]byte
	encodePathField(&path, "/sys/bus/usb/devices/1-2")

	entries := []DeviceEntry{{
		Info: DeviceInfoTruncated{
			Path:           path,
			BusNum:         1,
			DevNum:         2,
			Speed:          SpeedHigh,
			IDVendor:       0x1d6b,
			IDProduct:      0x0002,
			BNumInterfaces: 2,
		},
		Interfaces: []DeviceInterface{
			{BInterfaceClass: 0xff, BInterfaceSubClass: 1, BInterfaceProtocol: 2},
			{BInterfaceClass: 0x03, BInterfaceSubClass: 1, BInterfaceProtocol: 1},
		},
	}}
	copy(entries[0].Info.BusID[:], "1-2")

	var buffer bytes.Buffer
	require.NoError(t, WriteOpRepDevList(&buffer, entries))

	header, err := ReadOpHeader(&buffer)
	require.NoError(t, err)
	require.Equal(t, OpRepDevList, header.Code)
	require.Equal(t, OpStatusOK, header.Status)

	parsed, err := ReadOpRepDevListBody(&buffer)
	require.NoError(t, err)
	require.Equal(t, entries, parsed)
}

func TestReadOpRepDevListBodyRejectsTooManyEntries(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	require.NoError(t, binary.Write(&buffer, binary.BigEndian, uint32(maxOpRepDevListEntries+1)))

	_, err := ReadOpRepDevListBody(&buffer)
	require.ErrorContains(t, err, "device count too large")
}

func TestDeviceInfoHelpers(t *testing.T) {
	t.Parallel()

	var info DeviceInfoTruncated
	encodePathField(&info.Path, "/sys/bus/usb/devices/1-2")
	copy(info.BusID[:], "1-2")
	info.BusNum = 3
	info.DevNum = 9

	require.Equal(t, "/sys/bus/usb/devices/1-2", info.PathString())
	require.Empty(t, info.SerialString())
	require.Equal(t, "1-2", info.BusIDString())
	require.Equal(t, uint32(0x00030009), info.DevID())
}

func TestDeviceInfoV2RoundTrip(t *testing.T) {
	t.Parallel()

	var path [256]byte
	encodePathField(&path, "/sys/bus/usb/devices/1-2")
	entry := DeviceEntry{
		Info: DeviceInfoTruncated{
			Path:                path,
			BusNum:              1,
			DevNum:              2,
			Speed:               SpeedSuper,
			IDVendor:            0x1d6b,
			IDProduct:           0x0002,
			BCDDevice:           0x0100,
			BDeviceClass:        0xff,
			BDeviceSubClass:     1,
			BDeviceProtocol:     2,
			BConfigurationValue: 1,
			BNumConfigurations:  1,
			BNumInterfaces:      1,
		},
		Interfaces: []DeviceInterface{{BInterfaceClass: 0xff, BInterfaceSubClass: 1, BInterfaceProtocol: 2}},
		Serial:     "serial-1",
	}
	copy(entry.Info.BusID[:], "1-2")

	info := deviceInfoV2FromEntry(entry, "linux-sysfs", "usb:1d6b:0002:serial-1", deviceStateAvailable, 1, "available")
	require.Equal(t, "1-2", info.BusID)
	require.Equal(t, "serial-1", info.Serial)
	require.True(t, info.available())
	require.Equal(t, DeviceKey{BusID: "1-2", VendorID: 0x1d6b, ProductID: 0x0002, Serial: "serial-1"}, info.key())
	roundTrip := info.toDeviceEntry()
	require.Equal(t, "1-2", roundTrip.Info.BusIDString())
	require.Equal(t, "serial-1", roundTrip.Serial)
	require.Empty(t, roundTrip.Info.SerialString())
	require.Equal(t, uint16(0x1d6b), roundTrip.Info.IDVendor)
	require.Equal(t, uint16(0x0002), roundTrip.Info.IDProduct)
	require.Equal(t, entry.Interfaces, roundTrip.Interfaces)
}

func TestEncodePathFieldZeroPadsPath(t *testing.T) {
	t.Parallel()

	var info DeviceInfoTruncated
	encodePathField(&info.Path, "/sys/bus/usb/devices/1-2")

	pathLen := len("/sys/bus/usb/devices/1-2")
	require.Equal(t, byte(0), info.Path[pathLen])
	require.Equal(t, make([]byte, len(info.Path)-pathLen-1), info.Path[pathLen+1:])
	require.Empty(t, info.SerialString())
}

func TestMatches(t *testing.T) {
	t.Parallel()

	device := DeviceKey{
		BusID:     "1-2",
		VendorID:  0x1d6b,
		ProductID: 0x0002,
		Serial:    "serial-1",
	}

	testCases := []struct {
		name     string
		match    option.USBIPDeviceMatch
		expected bool
	}{
		{name: "zero", match: option.USBIPDeviceMatch{}, expected: false},
		{name: "busid", match: option.USBIPDeviceMatch{BusID: "1-2"}, expected: true},
		{name: "vendor-and-product", match: option.USBIPDeviceMatch{VendorID: 0x1d6b, ProductID: 0x0002}, expected: true},
		{name: "serial", match: option.USBIPDeviceMatch{Serial: "serial-1"}, expected: true},
		{name: "all-fields", match: option.USBIPDeviceMatch{BusID: "1-2", VendorID: 0x1d6b, ProductID: 0x0002, Serial: "serial-1"}, expected: true},
		{name: "vendor-mismatch", match: option.USBIPDeviceMatch{VendorID: 0x1d6c}, expected: false},
		{name: "serial-mismatch", match: option.USBIPDeviceMatch{Serial: "other"}, expected: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, testCase.expected, matches(testCase.match, device))
		})
	}
}
