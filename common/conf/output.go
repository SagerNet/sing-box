package conf

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/pelletier/go-toml"
	"github.com/sagernet/sing-box/common/conf/mergers"
	"gopkg.in/yaml.v2"
)

// Sprint returns the text of give format for v
func Sprint(v interface{}, format mergers.Format) (string, error) {
	var (
		out []byte
		err error
	)
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(v)
	if err != nil {
		return "", fmt.Errorf("failed to convert to json: %s", err)
	}
	if format == mergers.FormatJSON || format == mergers.FormatJSONC {
		return string(buffer.Bytes()), nil
	}

	m := make(map[string]interface{})
	err = json.Unmarshal(buffer.Bytes(), &m)
	if err != nil {
		return "", err
	}
	switch format {
	case mergers.FormatTOML:
		out, err = toml.Marshal(m)
		if err != nil {
			return "", fmt.Errorf("failed to convert to toml: %s", err)
		}
	case mergers.FormatYAML:
		out, err = yaml.Marshal(m)
		if err != nil {
			return "", fmt.Errorf("failed to convert to yaml: %s", err)
		}
	default:
		return "", fmt.Errorf("invalid output format: %s", format)
	}
	return string(out), nil
}
