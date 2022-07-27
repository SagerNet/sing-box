package badjson

import (
	"bytes"

	"github.com/sagernet/sing-box/common/json"
	E "github.com/sagernet/sing/common/exceptions"
)

func Decode(content []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(content))
	return decodeJSON(decoder)
}

func decodeJSON(decoder *json.Decoder) (any, error) {
	rawToken, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch token := rawToken.(type) {
	case json.Delim:
		switch token {
		case '{':
			var object JSONObject
			err = object.decodeJSON(decoder)
			if err != nil {
				return nil, err
			}
			rawToken, err = decoder.Token()
			if err != nil {
				return nil, err
			} else if rawToken != json.Delim('}') {
				return nil, E.New("excepted object end, but got ", rawToken)
			}
			return &object, nil
		case '[':
			var array JSONArray
			err = array.decodeJSON(decoder)
			if err != nil {
				return nil, err
			}
			rawToken, err = decoder.Token()
			if err != nil {
				return nil, err
			} else if rawToken != json.Delim(']') {
				return nil, E.New("excepted array end, but got ", rawToken)
			}
			return array, nil
		default:
			return nil, E.New("excepted object or array end: ", token)
		}
	}
	return rawToken, nil
}
