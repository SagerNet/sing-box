package option

import (
	"encoding/json"
	"net/netip"
)

type ListenAddress netip.Addr

func (a *ListenAddress) MarshalJSON() ([]byte, error) {
	value := netip.Addr(*a).String()
	return json.Marshal(value)
}

func (a *ListenAddress) UnmarshalJSON(bytes []byte) error {
	var value string
	err := json.Unmarshal(bytes, &value)
	if err != nil {
		return err
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return err
	}
	*a = ListenAddress(addr)
	return nil
}
