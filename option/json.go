package option

import (
	"encoding/json"
)

func ToMap(v any) (map[string]any, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var content map[string]any
	err = json.Unmarshal(bytes, &content)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func MergeObjects(objects ...any) (map[string]any, error) {
	content := make(map[string]any)
	for _, object := range objects {
		objectMap, err := ToMap(object)
		if err != nil {
			return nil, err
		}
		for k, v := range objectMap {
			content[k] = v
		}
	}
	return content, nil
}

func MarshallObjects(objects ...any) ([]byte, error) {
	content, err := MergeObjects(objects...)
	if err != nil {
		return nil, err
	}
	return json.Marshal(content)
}
