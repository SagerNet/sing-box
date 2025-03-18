package option

import (
	"encoding/json"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badjson"
)

type ShadowTLSInboundOptions struct {
	ListenOptions
	Version                int                                                  `json:"version,omitempty"`
	Password               string                                               `json:"password,omitempty"`
	Users                  []ShadowTLSUser                                      `json:"users,omitempty"`
	Handshake              ShadowTLSHandshakeOptions                            `json:"handshake,omitempty"`
	HandshakeForServerName *badjson.TypedMap[string, ShadowTLSHandshakeOptions] `json:"handshake_for_server_name,omitempty"`
	StrictMode             bool                                                 `json:"strict_mode,omitempty"`
	WildcardSNI            WildcardSNI                                          `json:"wildcard_sni,omitempty"`
}

type WildcardSNI int

const (
	ShadowTLSWildcardSNIOff WildcardSNI = iota
	ShadowTLSWildcardSNIAuthed
	ShadowTLSWildcardSNIAll
)

func (w WildcardSNI) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.String())
}

func (w WildcardSNI) String() string {
	switch w {
	case ShadowTLSWildcardSNIOff:
		return "off"
	case ShadowTLSWildcardSNIAuthed:
		return "authed"
	case ShadowTLSWildcardSNIAll:
		return "all"
	default:
		panic("unknown wildcard SNI value")
	}
}

func (w *WildcardSNI) UnmarshalJSON(bytes []byte) error {
	var valueString string
	err := json.Unmarshal(bytes, &valueString)
	if err != nil {
		return err
	}
	switch valueString {
	case "off", "":
		*w = ShadowTLSWildcardSNIOff
	case "authed":
		*w = ShadowTLSWildcardSNIAuthed
	case "all":
		*w = ShadowTLSWildcardSNIAll
	default:
		return E.New("unknown wildcard SNI value: ", valueString)
	}
	return nil
}

type ShadowTLSUser struct {
	Name     string `json:"name,omitempty"`
	Password string `json:"password,omitempty"`
}

type ShadowTLSHandshakeOptions struct {
	ServerOptions
	DialerOptions
}

type ShadowTLSOutboundOptions struct {
	DialerOptions
	ServerOptions
	Version  int    `json:"version,omitempty"`
	Password string `json:"password,omitempty"`
	OutboundTLSOptionsContainer
}
