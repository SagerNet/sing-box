//go:build windows

package usbip

import (
	"errors"

	"github.com/sagernet/sing-box/common/vboxusb"
	E "github.com/sagernet/sing/common/exceptions"
)

// vboxusbEngine adapts a vboxusb.Device into the URBEngine interface.
// It owns the device handle for the lifetime of one userspaceURBSession
// (closed via Close).
type vboxusbEngine struct {
	device *vboxusb.Device
}

func newVBoxUSBEngine(device *vboxusb.Device) *vboxusbEngine {
	return &vboxusbEngine{device: device}
}

func (e *vboxusbEngine) Submit(req URBRequest) URBResponse {
	command := req.Command
	// EP0 traps: SET_CONFIGURATION, SET_INTERFACE, CLEAR_FEATURE(ENDPOINT_HALT)
	// must go through dedicated IOCTLs because VBoxUSB rebuilds its
	// pipe-handle table on the first two and resets the host-side pipe
	// state on the third — racing a SEND_URB against them produces
	// wrong-pipe errors. Mirrors usbipd-win AttachedEndpoint.cs:270-319.
	if command.Header.Endpoint == 0 {
		response, trapped := e.trapStandardControl(command)
		if trapped {
			return response
		}
		return e.controlSubmit(req)
	}
	if command.NumberOfPackets > 0 {
		return e.isoSubmit(req)
	}
	transferType := vboxusb.TransferTypeBulk
	if command.Interval > 0 {
		transferType = vboxusb.TransferTypeInterrupt
	}
	return e.bulkSubmit(req, transferType)
}

func (e *vboxusbEngine) AbortEndpoint(endpoint uint8) error {
	return e.device.AbortEndpoint(endpoint)
}

func (e *vboxusbEngine) Close() error {
	return e.device.Close()
}

// trapStandardControl returns (response, true) when the EP0 transfer
// is one VBoxUSB requires us to translate. (zero, false) means the
// caller should proceed with a normal control SEND_URB.
func (e *vboxusbEngine) trapStandardControl(command SubmitCommand) (URBResponse, bool) {
	bmRequestType := command.Setup[0]
	bRequest := command.Setup[1]
	wValue := uint16(command.Setup[2]) | uint16(command.Setup[3])<<8
	wIndex := uint16(command.Setup[4]) | uint16(command.Setup[5])<<8

	switch {
	case bmRequestType == 0x00 && bRequest == 0x09: // SET_CONFIGURATION
		err := e.device.SetConfig(byte(wValue))
		return controlAckResponse(err), true
	case bmRequestType == 0x01 && bRequest == 0x0b: // SET_INTERFACE
		err := e.device.SelectInterface(byte(wIndex), byte(wValue))
		return controlAckResponse(err), true
	case bmRequestType == 0x02 && bRequest == 0x01 && wValue == 0x00: // CLEAR_FEATURE(ENDPOINT_HALT)
		err := e.device.ClearEndpoint(byte(wIndex))
		return controlAckResponse(err), true
	}
	return URBResponse{}, false
}

func controlAckResponse(err error) URBResponse {
	if err != nil {
		return URBResponse{Error: err}
	}
	return URBResponse{Status: 0, ActualLength: 0}
}

// controlSubmit sends a non-trapped EP0 transfer as USBSUP_TRANSFER_TYPE_MSG
// with the 8-byte setup packet prepended to the buffer; urb.Length
// includes those 8 bytes. The reply strips them back off so the
// session sees only the data stage.
func (e *vboxusbEngine) controlSubmit(req URBRequest) URBResponse {
	command := req.Command
	data := req.Buffer
	combined := make([]byte, 8+len(data))
	copy(combined[:8], command.Setup[:])
	if command.Header.Direction == USBIPDirOut && len(data) > 0 {
		copy(combined[8:], data)
	}
	urb := &vboxusb.URB{
		Type:      vboxusb.TransferTypeMessage,
		Endpoint:  0,
		Direction: vboxusb.DirectionSetup,
		Length:    uint64(len(combined)),
		Buffer:    combined,
	}
	err := e.device.SendURB(urb)
	status, ok := classifyURBError(err)
	if !ok {
		return URBResponse{Error: err}
	}
	actual := int64(urb.Length) - 8
	if actual < 0 {
		actual = 0
	}
	resp := URBResponse{Status: status, ActualLength: int32(actual)}
	if command.Header.Direction == USBIPDirIn && actual > 0 {
		end := 8 + int(actual)
		if end > len(combined) {
			end = len(combined)
		}
		resp.Buffer = combined[8:end]
	}
	return resp
}

func (e *vboxusbEngine) bulkSubmit(req URBRequest, transferType vboxusb.TransferType) URBResponse {
	command := req.Command
	urb := &vboxusb.URB{
		Type:      transferType,
		Endpoint:  uint32(req.Endpoint & 0x0f),
		Direction: directionFromCommand(command.Header.Direction),
		Flags:     flagsFromCommand(command.TransferFlags, command.Header.Direction),
		Length:    uint64(len(req.Buffer)),
		Buffer:    req.Buffer,
	}
	err := e.device.SendURB(urb)
	status, ok := classifyURBError(err)
	if !ok {
		return URBResponse{Error: err}
	}
	resp := URBResponse{Status: status, ActualLength: int32(urb.Length)}
	if command.Header.Direction == USBIPDirIn && urb.Length > 0 {
		end := int(urb.Length)
		if end > len(req.Buffer) {
			end = len(req.Buffer)
		}
		resp.Buffer = req.Buffer[:end]
	}
	return resp
}

// isoSubmit currently rejects iso transfers exceeding VBoxUSB's 8-
// packet-per-URB limit. usbipd-win splits these into multiple
// parallel SEND_URB calls sharing one pinned buffer; that splitter is
// a Phase C follow-up. Single-shot iso under 8 packets does work.
func (e *vboxusbEngine) isoSubmit(req URBRequest) URBResponse {
	command := req.Command
	if len(command.IsoPackets) > vboxusb.MaxIsoPacketsPerURB {
		return URBResponse{Error: E.New("vboxusb iso submit: ", len(command.IsoPackets), " packets exceeds per-URB limit; iso splitter not yet implemented")}
	}
	pkts := make([]vboxusb.IsoPacket, len(command.IsoPackets))
	for i, p := range command.IsoPackets {
		pkts[i] = vboxusb.IsoPacket{
			Length: uint16(p.Length),
			Offset: uint16(p.Offset),
		}
	}
	urb := &vboxusb.URB{
		Type:       vboxusb.TransferTypeIso,
		Endpoint:   uint32(req.Endpoint & 0x0f),
		Direction:  directionFromCommand(command.Header.Direction),
		Flags:      flagsFromCommand(command.TransferFlags, command.Header.Direction),
		Length:     uint64(len(req.Buffer)),
		Buffer:     req.Buffer,
		IsoPackets: pkts,
	}
	err := e.device.SendURB(urb)
	status, ok := classifyURBError(err)
	if !ok {
		return URBResponse{Error: err}
	}
	for i := range req.IsoPackets {
		if i >= len(urb.IsoPackets) {
			break
		}
		req.IsoPackets[i].ActualLength = int32(urb.IsoPackets[i].Length)
		req.IsoPackets[i].Status = vboxusbStatusToUSBIP(urb.IsoPackets[i].Status)
	}
	resp := URBResponse{Status: status, ActualLength: int32(urb.Length), IsoPackets: req.IsoPackets}
	if command.Header.Direction == USBIPDirIn && urb.Length > 0 {
		end := int(urb.Length)
		if end > len(req.Buffer) {
			end = len(req.Buffer)
		}
		resp.Buffer = req.Buffer[:end]
	}
	return resp
}

func directionFromCommand(usbipDir uint32) vboxusb.Direction {
	if usbipDir == USBIPDirIn {
		return vboxusb.DirectionIn
	}
	return vboxusb.DirectionOut
}

// flagsFromCommand sets SHORT_OK on IN transfers unless USB/IP set
// URB_SHORT_NOT_OK (bit 0). Matches usbipd-win AttachedEndpoint.cs:224-226.
func flagsFromCommand(transferFlags int32, usbipDir uint32) vboxusb.TransferFlags {
	if usbipDir != USBIPDirIn {
		return vboxusb.TransferFlagNone
	}
	const usbipShortNotOK int32 = 0x00000001
	if transferFlags&usbipShortNotOK != 0 {
		return vboxusb.TransferFlagNone
	}
	return vboxusb.TransferFlagShortOK
}

// classifyURBError returns (status, ok). ok=true means the URB completed
// (with possibly a non-zero device-level status); ok=false means a
// transport failure that the caller surfaces as URBResponse.Error.
func classifyURBError(err error) (int32, bool) {
	if err == nil {
		return 0, true
	}
	var statusErr *vboxusb.URBStatusError
	if errors.As(err, &statusErr) {
		return vboxusbStatusToUSBIP(statusErr.Code), true
	}
	return 0, false
}

// vboxusbStatusToUSBIP maps USBSUP_ERROR onto the USBIP (Linux errno)
// wire status convention.
func vboxusbStatusToUSBIP(code vboxusb.URBError) int32 {
	switch code {
	case vboxusb.URBOK:
		return 0
	case vboxusb.URBStall:
		return -32 // EPIPE
	case vboxusb.URBDeviceNotResponding:
		return -19 // ENODEV
	case vboxusb.URBCRCError, vboxusb.URBNACError:
		return -71 // EPROTO
	case vboxusb.URBUnderrun, vboxusb.URBOverrun:
		return -75 // EOVERFLOW
	default:
		return usbipStatusEIO
	}
}
