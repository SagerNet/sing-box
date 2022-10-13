package mergers

import (
	"strings"
)

// Format is the supported format of mergers
type Format string

// Supported formats
const (
	FormatAuto  Format = "auto"
	FormatJSON  Format = "json"
	FormatJSONC Format = "jsonc"
	FormatTOML  Format = "toml"
	FormatYAML  Format = "yaml"
)

// ParseFormat parses format from name, it returns FormatAuto if fails.
func ParseFormat(name string) Format {
	name = strings.ToLower(strings.TrimSpace(name))
	format := Format(name)
	switch format {
	case FormatAuto, FormatJSON, FormatJSONC, FormatTOML, FormatYAML:
		return format
	default:
		return FormatAuto
	}
}
