package mergers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sagernet/sing-box/common/conf/jsonc"
	"github.com/sagernet/sing/common"
)

func init() {
	common.Must(registerMerger(makeMerger(
		FormatJSON,
		[]string{".json"},
		func(v []byte) (map[string]interface{}, error) {
			m := make(map[string]interface{})
			if err := json.Unmarshal(v, &m); err != nil {
				return nil, err
			}
			return m, nil
		},
	)))
	common.Must(registerMerger(makeMerger(
		FormatJSONC,
		[]string{".jsonc"},
		func(v []byte) (map[string]interface{}, error) {
			m := make(map[string]interface{})
			if err := jsonc.Decode(bytes.NewReader(v), &m); err != nil {
				return nil, err
			}
			return m, nil
		},
	)))
	common.Must(registerMerger(
		&Merger{
			Name:       FormatAuto,
			Extensions: nil,
			Merge:      Merge,
		}),
	)
}

var (
	mergersByName = make(map[Format]*Merger)
	mergersByExt  = make(map[string]*Merger)
)

// registerMerger add a new Merger.
func registerMerger(format *Merger) error {
	if _, found := mergersByName[format.Name]; found {
		return fmt.Errorf("%s already registered", format.Name)
	}
	mergersByName[format.Name] = format
	for _, ext := range format.Extensions {
		lext := strings.ToLower(ext)
		if f, found := mergersByExt[lext]; found {
			return fmt.Errorf("%s already registered to %s", ext, f.Name)
		}
		mergersByExt[lext] = format
	}
	return nil
}
