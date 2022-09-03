package option

type TorOutboundOptions struct {
	DialerOptions
	ExecutablePath string            `json:"executable_path,omitempty"`
	ExtraArgs      []string          `json:"extra_args,omitempty"`
	DataDirectory  string            `json:"data_directory,omitempty"`
	Options        map[string]string `json:"torrc,omitempty"`
}
