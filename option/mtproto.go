package option

type MTProtoInboundOptions struct {
	ListenOptions
	Users []MTProtoUser `json:"users,omitempty"`
}

type MTProtoUser struct {
	Name   string `json:"name,omitempty"`
	Secret string `json:"secret"`
}
