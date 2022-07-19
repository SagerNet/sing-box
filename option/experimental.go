package option

type ExperimentalOptions struct {
	ClashAPI *ClashAPIOptions `json:"clash_api,omitempty"`
}

type ClashAPIOptions struct {
	ExternalController string `json:"external_controller,omitempty"`
	ExternalUI         string `json:"external_ui,omitempty"`
	Secret             string `json:"secret,omitempty"`
}
