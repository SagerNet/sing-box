package merge

import (
	"fmt"
)

// Convert converts map[interface{}]interface{} to map[string]interface{} which
// is mergable by merge.Maps
func Convert(m map[interface{}]interface{}) map[string]interface{} {
	return convert(m)
}

func convert(m map[interface{}]interface{}) map[string]interface{} {
	res := map[string]interface{}{}
	for k, v := range m {
		var value interface{}
		switch v2 := v.(type) {
		case map[interface{}]interface{}:
			value = convert(v2)
		case []interface{}:
			for i, el := range v2 {
				if m, ok := el.(map[interface{}]interface{}); ok {
					v2[i] = convert(m)
				}
			}
			value = v2
		default:
			value = v
		}
		key := "null"
		if k != nil {
			key = fmt.Sprint(k)
		}
		res[key] = value
	}
	return res
}
