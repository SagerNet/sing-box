package mergers

import (
	"fmt"
)

// GetExtensions get extensions of given format
func GetExtensions(formatName Format) ([]string, error) {
	if formatName == FormatAuto {
		return GetAllExtensions(), nil
	}
	f, found := mergersByName[formatName]
	if !found {
		return nil, fmt.Errorf("%s not found", formatName)
	}
	return f.Extensions, nil
}

// GetAllExtensions get all extensions supported
func GetAllExtensions() []string {
	extensions := make([]string, 0)
	for _, f := range mergersByName {
		extensions = append(extensions, f.Extensions...)
	}
	return extensions
}
