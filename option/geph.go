package option

import "github.com/sagernet/sing/common/json/badoption"

type GephEndpointOptions struct {
	ExecutablePath string             `json:"executable_path,omitempty"`
	ConfigPath     string             `json:"config_path"`
	ExtraArgs      []string           `json:"extra_args,omitempty"`
	StartupTimeout badoption.Duration `json:"startup_timeout,omitempty"`
}
