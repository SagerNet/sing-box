package option

import "github.com/sagernet/sing/common/json/badoption"

type SSHOutboundOptions struct {
	DialerOptions
	ServerOptions
	User                 string                     `json:"user,omitempty"`
	Password             string                     `json:"password,omitempty"`
	PrivateKey           badoption.Listable[string] `json:"private_key,omitempty"`
	PrivateKeyPath       string                     `json:"private_key_path,omitempty"`
	PrivateKeyPassphrase string                     `json:"private_key_passphrase,omitempty"`
	HostKey              badoption.Listable[string] `json:"host_key,omitempty"`
	HostKeyAlgorithms    badoption.Listable[string] `json:"host_key_algorithms,omitempty"`
	ClientVersion        string                     `json:"client_version,omitempty"`
}
