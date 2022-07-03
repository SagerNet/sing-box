package option

import (
	"bytes"

	"github.com/goccy/go-json"
	"github.com/sagernet/sing-box/common/badjson"
)

func ToMap(v any) (*badjson.JSONObject, error) {
	inputContent, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var content badjson.JSONObject
	err = content.UnmarshalJSON(inputContent)
	if err != nil {
		return nil, err
	}
	return &content, nil
}

func MergeObjects(objects ...any) (*badjson.JSONObject, error) {
	var content badjson.JSONObject
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
	var content badjson.JSONObject
	err = content.UnmarshalJSON(inputContent)
	if err != nil {
		return err
	}
	for _, key := range parentContent.Keys() {
		content.Remove(key)
	}
	inputContent, err = content.MarshalJSON()
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(inputContent))
	decoder.DisallowUnknownFields()
	return decoder.Decode(object)
}
