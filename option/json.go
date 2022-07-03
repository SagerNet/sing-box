package option

import (
	"bytes"

	"github.com/goccy/go-json"
	"github.com/sagernet/sing-box/common/linkedhashmap"
)

func ToMap(v any) (*linkedhashmap.Map[string, any], error) {
	inputContent, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var content linkedhashmap.Map[string, any]
	err = content.UnmarshalJSON(inputContent)
	if err != nil {
		return nil, err
	}
	return &content, nil
}

func MergeObjects(objects ...any) (*linkedhashmap.Map[string, any], error) {
	var content linkedhashmap.Map[string, any]
	for _, object := range objects {
		objectMap, err := ToMap(object)
		if err != nil {
			return nil, err
		}
		content.PutAll(objectMap)
	}
	return &content, nil
}

func MarshallObjects(objects ...any) ([]byte, error) {
	content, err := MergeObjects(objects...)
	if err != nil {
		return nil, err
	}
	return content.MarshalJSON()
}

func UnmarshallExcluded(inputContent []byte, parentObject any, object any) error {
	parentContent, err := ToMap(parentObject)
	if err != nil {
		return err
	}
	var content linkedhashmap.Map[string, any]
	err = content.UnmarshalJSON(inputContent)
	if err != nil {
		return err
	}
	content.RemoveAll(parentContent.Keys())
	inputContent, err = content.MarshalJSON()
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(inputContent))
	decoder.DisallowUnknownFields()
	return decoder.Decode(object)
}
