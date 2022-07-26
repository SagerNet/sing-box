package badjson

import (
	"bytes"
	"reflect"

	"github.com/sagernet/sing-box/common/json"
	E "github.com/sagernet/sing/common/exceptions"
)

type JSONArray[T any] []T

func (a JSONArray[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal([]T(a))
}

func (a *JSONArray[T]) UnmarshalJSON(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	arrayStart, err := decoder.Token()
	if err != nil {
		return err
	} else if arrayStart != json.Delim('[') {
		return E.New("excepted array start, but got ", arrayStart)
	}
	err = a.decodeJSON(decoder)
	if err != nil {
		return err
	}
	arrayEnd, err := decoder.Token()
	if err != nil {
		return err
	} else if arrayEnd != json.Delim(']') {
		return E.New("excepted array end, but got ", arrayEnd)
	}
	return nil
}

func (a *JSONArray[T]) decodeJSON(decoder *json.Decoder) error {
	for decoder.More() {
		value, err := decodeJSON(decoder)
		if err != nil {
			return err
		}
		item, ok := value.(T)
		if !ok {
			var defValue T
			return E.New("can't cast ", value, " to ", reflect.TypeOf(defValue))
		}
		*a = append(*a, item)
	}
	return nil
}
