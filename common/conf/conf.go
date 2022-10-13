package conf

import (
	"encoding/json"
	"io"

	"github.com/sagernet/sing-box/common/conf/merge"
	"github.com/sagernet/sing-box/common/conf/mergers"
)

// Reader loads json data to v.
func Reader(v interface{}, r io.Reader, format mergers.Format) error {
	data, err := ReaderToJSON(r, format)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// Files load config files to v.
// it will resolve folder to files
func Files(v interface{}, files []string, format mergers.Format, recursively bool) error {
	data, err := FilesToJSON(files, format, recursively)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// ReaderToJSON load reader content as JSON bytes.
func ReaderToJSON(r io.Reader, format mergers.Format) ([]byte, error) {
	m := make(map[string]interface{})
	err := mergers.MergeAs(format, r, m)
	if err != nil {
		return nil, err
	}
	err = merge.ApplyRules(m)
	if err != nil {
		return nil, err
	}
	merge.RemoveHelperFields(m)
	return json.Marshal(m)
}

// FilesToJSON merges config files JSON bytes
func FilesToJSON(files []string, format mergers.Format, recursively bool) ([]byte, error) {
	var err error
	if len(files) > 0 {
		var extensions []string
		extensions, err := mergers.GetExtensions(format)
		if err != nil {
			return nil, err
		}
		files, err = resolveFolderToFiles(files, extensions, recursively)
		if err != nil {
			return nil, err
		}
	}
	m := make(map[string]interface{})
	if len(files) > 0 {
		err = mergers.MergeAs(format, files, m)
		if err != nil {
			return nil, err
		}
	}
	err = merge.ApplyRules(m)
	if err != nil {
		return nil, err
	}
	merge.RemoveHelperFields(m)
	return json.Marshal(m)
}
