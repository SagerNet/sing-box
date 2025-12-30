package option

type SudokuInboundOptions struct {
	ListenOptions
	Key                string `json:"key"`
	AEADMethod         string `json:"aead,omitempty"`
	PaddingMin         *int   `json:"padding_min,omitempty"`
	PaddingMax         *int   `json:"padding_max,omitempty"`
	ASCII              string `json:"ascii,omitempty"`
	CustomTable        string `json:"custom_table,omitempty"`
	CustomTables       []string `json:"custom_tables,omitempty"`
	EnablePureDownlink *bool  `json:"enable_pure_downlink,omitempty"`
	HandshakeTimeout   int    `json:"handshake_timeout,omitempty"`
	DisableHTTPMask    bool   `json:"disable_http_mask,omitempty"`
	HTTPMaskMode       string `json:"http_mask_mode,omitempty"`
}

type SudokuOutboundOptions struct {
	DialerOptions
	ServerOptions
	Key                string   `json:"key"`
	AEADMethod         string   `json:"aead,omitempty"`
	PaddingMin         *int     `json:"padding_min,omitempty"`
	PaddingMax         *int     `json:"padding_max,omitempty"`
	ASCII              string   `json:"ascii,omitempty"`
	CustomTable        string   `json:"custom_table,omitempty"`
	CustomTables       []string `json:"custom_tables,omitempty"`
	EnablePureDownlink *bool    `json:"enable_pure_downlink,omitempty"`
	DisableHTTPMask    bool     `json:"disable_http_mask,omitempty"`
	HTTPMaskMode       string   `json:"http_mask_mode,omitempty"`
	HTTPMaskTLS        bool     `json:"http_mask_tls,omitempty"`
	HTTPMaskHost       string   `json:"http_mask_host,omitempty"`
	HTTPMaskStrategy   string   `json:"http_mask_strategy,omitempty"`
}

