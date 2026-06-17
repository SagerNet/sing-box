package option

import (
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
)

const (
	USBIPProviderDefault = "default"
	USBIPProviderDynamic = "dynamic"
)

type _USBIPServerServiceOptions struct {
	ListenOptions
	Provider string `json:"provider,omitempty"`
	Options  any    `json:"-"`
}

type USBIPServerServiceOptions _USBIPServerServiceOptions

func (o USBIPServerServiceOptions) MarshalJSON() ([]byte, error) {
	if o.Options == nil {
		return json.Marshal((_USBIPServerServiceOptions)(o))
	}
	return badjson.MarshallObjects((_USBIPServerServiceOptions)(o), o.Options)
}

func (o *USBIPServerServiceOptions) UnmarshalJSON(content []byte) error {
	err := json.Unmarshal(content, (*_USBIPServerServiceOptions)(o))
	if err != nil {
		return err
	}
	var options any
	switch o.Provider {
	case "", USBIPProviderDefault:
		o.Provider = USBIPProviderDefault
		options = new(USBIPDefaultProviderOptions)
	case USBIPProviderDynamic:
		options = new(USBIPDynamicProviderOptions)
	default:
		return E.New("unknown usbip provider type: ", o.Provider)
	}
	err = badjson.UnmarshallExcluded(content, (*_USBIPServerServiceOptions)(o), options)
	if err != nil {
		return err
	}
	o.Options = options
	return nil
}

type USBIPClientServiceOptions struct {
	DialerOptions
	ServerOptions
	Devices []USBIPDeviceMatch `json:"devices,omitempty"`
}

type USBIPDeviceMatch struct {
	BusID     string `json:"bus_id,omitempty"`
	VendorID  uint16 `json:"vendor_id,omitempty"`
	ProductID uint16 `json:"product_id,omitempty"`
	Serial    string `json:"serial,omitempty"`
}

type USBIPDefaultProviderOptions struct {
	Devices []USBIPDeviceMatch `json:"devices,omitempty"`
}

type USBIPDynamicProviderOptions struct{}
