package option

type TLSFragmentOptions struct {
	Enabled bool   `json:"enabled,omitempty"`
	Method  string `json:"method,omitempty"` // Wether to fragment only clientHello or a range of TCP packets. Valid options: ['tlsHello', 'range']
	Size    string `json:"size,omitempty"`   // Fragment size in Bytes
	Sleep   string `json:"sleep,omitempty"`  // Time to sleep between sending the fragments in milliseconds
	Range   string `json:"range,omitempty"`  // Range of packets to fragment, effective when 'method' is set to 'range'
}
