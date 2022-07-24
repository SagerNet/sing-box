//go:build !with_std_json

package json

import "github.com/goccy/go-json"

var (
	Marshal    = json.Marshal
	Unmarshal  = json.Unmarshal
	NewEncoder = json.NewEncoder
	NewDecoder = json.NewDecoder
)

type (
	Encoder     = json.Encoder
	Decoder     = json.Decoder
	Token       = json.Token
	Delim       = json.Delim
	SyntaxError = json.SyntaxError
)
