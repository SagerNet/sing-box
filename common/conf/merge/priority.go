// Copyright 2020 Jebbs. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package merge

import "sort"

func getPriority(v interface{}) float64 {
	var m map[string]interface{}
	var ok bool
	if m, ok = v.(map[string]interface{}); !ok {
		return 0
	}
	if i, ok := m[priorityKey]; ok {
		if p, ok := i.(float64); ok {
			return p
		}
	}
	return 0
}

// sortByPriority sort slice by priority fields of their elements
func sortByPriority(slice []interface{}) {
	sort.Slice(
		slice,
		func(i, j int) bool {
			return getPriority(slice[i]) < getPriority(slice[j])
		},
	)
}
