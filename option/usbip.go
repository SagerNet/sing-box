package option

import (
	"fmt"
	"strconv"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

// USBIPHexUint16 is a uint16 that accepts either a JSON integer or a hex
// string ("0x1d6b", "1d6b", "0X1D6B") on unmarshal, and emits a hex string
// on marshal. Zero means "unset".
type USBIPHexUint16 uint16

func (h USBIPHexUint16) MarshalJSON() ([]byte, error) {
	if h == 0 {
		return []byte(`""`), nil
	}
	return json.Marshal(fmt.Sprintf("0x%04x", uint16(h)))
}

func (h *USBIPHexUint16) UnmarshalJSON(data []byte) error {
	var asNumber uint64
	if err := json.Unmarshal(data, &asNumber); err == nil {
		if asNumber > 0xffff {
			return E.New("usb id out of uint16 range: ", asNumber)
		}
		*h = USBIPHexUint16(asNumber)
		return nil
	}
	var asString string
	if err := json.Unmarshal(data, &asString); err != nil {
		return E.Cause(err, "parse usb id")
	}
	asString = strings.TrimSpace(asString)
	if asString == "" {
		*h = 0
		return nil
	}
	parsed, err := strconv.ParseUint(asString, 0, 16)
	if err != nil {
		// Allow bare hex without 0x prefix.
		if parsed2, err2 := strconv.ParseUint(asString, 16, 16); err2 == nil {
			*h = USBIPHexUint16(parsed2)
			return nil
		}
		return E.Cause(err, "parse usb id ", asString)
	}
	*h = USBIPHexUint16(parsed)
	return nil
}

// USBIPDeviceMatch selects a USB device. Non-zero fields AND together.
// An all-zero match is rejected at service construction time.
type USBIPDeviceMatch struct {
	BusID     string         `json:"busid,omitempty"`
	VendorID  USBIPHexUint16 `json:"vendor_id,omitempty"`
	ProductID USBIPHexUint16 `json:"product_id,omitempty"`
	Serial    string         `json:"serial,omitempty"`
}

func (m USBIPDeviceMatch) IsZero() bool {
	return m.BusID == "" && m.VendorID == 0 && m.ProductID == 0 && m.Serial == ""
}

// USBIPServerServiceOptions configures a usbip-server service. It listens on
// TCP (default :3240) and binds matching local USB devices to the usbip-host
// kernel driver for export. Empty Devices means export nothing.
type USBIPServerServiceOptions struct {
	ListenOptions
	Devices []USBIPDeviceMatch `json:"devices,omitempty"`
}

// USBIPClientServiceOptions configures a usbip-client service. It connects to
// one remote usbip server and attaches matching remote USB devices to the
// local kernel via vhci_hcd. Empty Devices means import every device the
// remote currently exports.
type USBIPClientServiceOptions struct {
	ServerOptions
	DialerOptions
	Devices []USBIPDeviceMatch `json:"devices,omitempty"`
}
