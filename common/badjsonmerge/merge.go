package badjsonmerge

import (
	"encoding/json"
	"reflect"

	"github.com/sagernet/sing-box/common/badjson"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func MergeOptions(source option.Options, destination option.Options) (option.Options, error) {
	rawSource, err := json.Marshal(source)
	if err != nil {
		return option.Options{}, E.Cause(err, "marshal source")
	}
	rawDestination, err := json.Marshal(destination)
	if err != nil {
		return option.Options{}, E.Cause(err, "marshal destination")
	}
	rawMerged, err := MergeJSON(rawSource, rawDestination)
	if err != nil {
		return option.Options{}, E.Cause(err, "merge options")
	}
	var merged option.Options
	err = json.Unmarshal(rawMerged, &merged)
	if err != nil {
		return option.Options{}, E.Cause(err, "unmarshal merged options")
	}
	return merged, nil
}

func MergeJSON(rawSource json.RawMessage, rawDestination json.RawMessage) (json.RawMessage, error) {
	source, err := badjson.Decode(rawSource)
	if err != nil {
		return nil, E.Cause(err, "decode source")
	}
	destination, err := badjson.Decode(rawDestination)
	if err != nil {
		return nil, E.Cause(err, "decode destination")
	}
	merged, err := mergeJSON(source, destination)
	if err != nil {
		return nil, err
	}
	return json.Marshal(merged)
}

func mergeJSON(anySource any, anyDestination any) (any, error) {
	switch destination := anyDestination.(type) {
	case badjson.JSONArray:
		switch source := anySource.(type) {
		case badjson.JSONArray:
			destination = append(destination, source...)
		default:
			destination = append(destination, source)
		}
		return destination, nil
	case *badjson.JSONObject:
		switch source := anySource.(type) {
		case *badjson.JSONObject:
			for _, entry := range source.Entries() {
				oldValue, loaded := destination.Get(entry.Key)
				if loaded {
					var err error
					entry.Value, err = mergeJSON(entry.Value, oldValue)
					if err != nil {
						return nil, E.Cause(err, "merge object item ", entry.Key)
					}
				}
				destination.Put(entry.Key, entry.Value)
			}
		default:
			return nil, E.New("cannot merge json object into ", reflect.TypeOf(destination))
		}
		return destination, nil
	default:
		return destination, nil
	}
}
