package config

import (
	"encoding/json"
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
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

type ServerAddress M.Socksaddr

func (a *ServerAddress) MarshalJSON() ([]byte, error) {
	value := M.Socksaddr(*a).String()
	return json.Marshal(value)
}

func (a *ServerAddress) UnmarshalJSON(bytes []byte) error {
	var value string
	err := json.Unmarshal(bytes, &value)
	if err != nil {
		return err
	}
	if value == "" {
		return E.New("empty server address")
	}
	*a = ServerAddress(M.ParseSocksaddr(value))
	return nil
}
