package option

type ExperimentalOptions struct {
	ClashAPI *ClashAPIOptions `json:"clash_api,omitempty"`
}

type ClashAPIOptions struct {
	ExternalController string `json:"external_controller,omitempty"`
	Secret             string `json:"secret,omitempty"`
}
