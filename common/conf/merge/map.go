// Copyright 2020 Jebbs. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package merge

import (
	"fmt"
)

// Maps merges source maps into target
func Maps(target map[string]interface{}, sources ...map[string]interface{}) (err error) {
	for _, source := range sources {
		err = mergeMaps(target, source)
		if err != nil {
			return err
		}
	}
	return nil
}

// mergeMaps merges source map into target
// it supports only map[string]interface{} type for any children of the map tree
func mergeMaps(target map[string]interface{}, source map[string]interface{}) (err error) {
	for key, value := range source {
		target[key], err = mergeField(target[key], value)
		if err != nil {
			return
		}
	}
	return
}

func mergeField(target interface{}, source interface{}) (interface{}, error) {
	if source == nil {
		return target, nil
	}
	if target == nil {
		return source, nil
	}
	if slice, ok := source.([]interface{}); ok {
		if tslice, ok := target.([]interface{}); ok {
			tslice = append(tslice, slice...)
			return tslice, nil
		}
		return nil, fmt.Errorf("value type mismatch, source is 'slice' but target not: %s", source)
	} else if smap, ok := source.(map[string]interface{}); ok {
		if tmap, ok := target.(map[string]interface{}); ok {
			err := mergeMaps(tmap, smap)
			return tmap, err
		}
		return nil, fmt.Errorf("value type mismatch, source is 'map[string]interface{}' but target not: %s", source)
	}
	return source, nil
}
