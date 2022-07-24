package badjson

import (
	"bytes"

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
		var item T
		err := decoder.Decode(&item)
		if err != nil {
			return err
		}
		*a = append(*a, item)
	}
	return nil
}
