package option

import (
	"bytes"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/sagernet/sing-box/common/badjson"

	"github.com/goccy/go-json"
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
	objects = common.Filter(objects, func(v any) bool {
		return v != nil
	})
	if len(objects) == 1 {
		return json.Marshal(objects[0])
	}
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
	if object == nil {
		if content.IsEmpty() {
			return nil
		}
		return E.New("unexpected key: ", content.Keys()[0])
	}
	inputContent, err = content.MarshalJSON()
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(inputContent))
	decoder.DisallowUnknownFields()
	return decoder.Decode(object)
}
