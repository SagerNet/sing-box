package option

import (
	"fmt"
	"strconv"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

type USBIPHexUint16 uint16

func (h USBIPHexUint16) MarshalJSON() ([]byte, error) {
	if h == 0 {
		return []byte(`""`), nil
	}
	return json.Marshal(fmt.Sprintf("0x%04x", uint16(h)))
}

func (h *USBIPHexUint16) UnmarshalJSON(data []byte) error {
	var asNumber uint64
	err := json.Unmarshal(data, &asNumber)
	if err == nil {
		if asNumber > 0xffff {
			return E.New("usb id out of uint16 range: ", asNumber)
		}
		*h = USBIPHexUint16(asNumber)
		return nil
	}
	var asString string
	err = json.Unmarshal(data, &asString)
	if err != nil {
		return E.Cause(err, "parse usb id")
	}
	asString = strings.TrimSpace(asString)
	if asString == "" {
		*h = 0
		return nil
	}
	asString = strings.TrimPrefix(strings.TrimPrefix(asString, "0x"), "0X")
	parsed, err := strconv.ParseUint(asString, 16, 16)
	if err != nil {
		return E.Cause(err, "parse usb id ", asString)
	}
	*h = USBIPHexUint16(parsed)
	return nil
}

type USBIPDeviceMatch struct {
	BusID     string         `json:"busid,omitempty"`
	VendorID  USBIPHexUint16 `json:"vendor_id,omitempty"`
	ProductID USBIPHexUint16 `json:"product_id,omitempty"`
	Serial    string         `json:"serial,omitempty"`
}

func (m USBIPDeviceMatch) IsZero() bool {
	return m.BusID == "" && m.VendorID == 0 && m.ProductID == 0 && m.Serial == ""
}

type USBIPServerServiceOptions struct {
	ListenOptions
	Devices []USBIPDeviceMatch `json:"devices,omitempty"`
}

type USBIPClientServiceOptions struct {
	ServerOptions
	DialerOptions
	Devices []USBIPDeviceMatch `json:"devices,omitempty"`
}
