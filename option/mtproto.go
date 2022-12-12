package option

type MTProtoInboundOptions struct {
	ListenOptions
	Secret string `json:"secret"`
}
