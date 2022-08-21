package option

type SSHOutboundOptions struct {
	OutboundDialerOptions
	ServerOptions
	User                 string           `json:"user,omitempty"`
	Password             string           `json:"password,omitempty"`
	PrivateKey           string           `json:"private_key,omitempty"`
	PrivateKeyPath       string           `json:"private_key_path,omitempty"`
	PrivateKeyPassphrase string           `json:"private_key_passphrase,omitempty"`
	HostKeyAlgorithms    Listable[string] `json:"host_key_algorithms,omitempty"`
	ClientVersion        string           `json:"client_version,omitempty"`
}
