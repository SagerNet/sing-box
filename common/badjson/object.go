package badjson

import (
	"bytes"
	"strings"

	"github.com/sagernet/sing-box/common/json"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/x/linkedhashmap"
)

type JSONObject struct {
	linkedhashmap.Map[string, any]
}

func (m JSONObject) MarshalJSON() ([]byte, error) {
	buffer := new(bytes.Buffer)
	buffer.WriteString("{")
	items := m.Entries()
	iLen := len(items)
	for i, entry := range items {
		keyContent, err := json.Marshal(entry.Key)
		if err != nil {
			return nil, err
		}
		buffer.WriteString(strings.TrimSpace(string(keyContent)))
		buffer.WriteString(": ")
		valueContent, err := json.Marshal(entry.Value)
		if err != nil {
			return nil, err
		}
		buffer.WriteString(strings.TrimSpace(string(valueContent)))
		if i < iLen-1 {
			buffer.WriteString(", ")
		}
	}
	buffer.WriteString("}")
	return buffer.Bytes(), nil
}

func (m *JSONObject) UnmarshalJSON(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	m.Clear()
	objectStart, err := decoder.Token()
	if err != nil {
		return err
	} else if objectStart != json.Delim('{') {
		return E.New("expected json object start, but starts with ", objectStart)
	}
	err = m.decodeJSON(decoder)
	if err != nil {
		return E.Cause(err, "decode json object content")
	}
	objectEnd, err := decoder.Token()
	if err != nil {
		return err
	} else if objectEnd != json.Delim('}') {
		return E.New("expected json object end, but ends with ", objectEnd)
	}
	return nil
}

func (m *JSONObject) decodeJSON(decoder *json.Decoder) error {
	for decoder.More() {
		var entryKey string
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		entryKey = keyToken.(string)
		var entryValue any
		entryValue, err = decodeJSON(decoder)
		if err != nil {
			return E.Cause(err, "decode value for ", entryKey)
		}
		m.Put(entryKey, entryValue)
	}
	return nil
}
