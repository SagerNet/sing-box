package option

type ExperimentalOptions struct {
	ClashAPI *ClashAPIOptions `json:"clash_api,omitempty"`
	V2RayAPI *V2RayAPIOptions `json:"v2ray_api,omitempty"`
	Debug    *DebugOptions    `json:"debug,omitempty"`
}
