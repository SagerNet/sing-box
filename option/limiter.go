package option

type Limiter struct {
	Tag                 string           `json:"tag"`
	Download            string           `json:"download,omitempty"`
	Upload              string           `json:"upload,omitempty"`
	AuthUser            Listable[string] `json:"auth_user,omitempty"`
	AuthUserIndependent bool             `json:"auth_user_independent,omitempty"`
	Inbound             Listable[string] `json:"inbound,omitempty"`
	InboundIndependent  bool             `json:"inbound_independent,omitempty"`
}
