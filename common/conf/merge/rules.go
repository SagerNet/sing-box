// Copyright 2020 Jebbs. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package merge

const priorityKey string = "_priority"
const tagKey string = "_tag"

// ApplyRules applies merge rules according to _tag, _priority fields
func ApplyRules(m map[string]interface{}) error {
	return sortMergeSlices(m)
}

// RemoveHelperFields removes helper fields from a map, including "_priority" and "_tag"
func RemoveHelperFields(target map[string]interface{}) {
	removeHelperFields(target)
}

// sortMergeSlices enumerates all slices in a map, to sort by priority and merge by tag
func sortMergeSlices(target map[string]interface{}) error {
	for key, value := range target {
		if slice, ok := value.([]interface{}); ok {
			sortByPriority(slice)
			s, err := mergeSameTag(slice)
			if err != nil {
				return err
			}
			target[key] = s
			for _, item := range s {
				if m, ok := item.(map[string]interface{}); ok {
					sortMergeSlices(m)
				}
			}
		} else if field, ok := value.(map[string]interface{}); ok {
			sortMergeSlices(field)
		}
	}
	return nil
}

func removeHelperFields(target map[string]interface{}) {
	for key, value := range target {
		if key == priorityKey || key == tagKey {
			delete(target, key)
		} else if slice, ok := value.([]interface{}); ok {
			for _, e := range slice {
				if el, ok := e.(map[string]interface{}); ok {
					removeHelperFields(el)
				}
			}
		} else if field, ok := value.(map[string]interface{}); ok {
			removeHelperFields(field)
		}
	}
}
