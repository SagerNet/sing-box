// Copyright 2020 Jebbs. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package merge_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/common/conf/jsonc"
	"github.com/sagernet/sing-box/common/conf/merge"
)

func TestMergeV2Style(t *testing.T) {
	json1 := `
	{
	  "log": {"level": "debug"},
	  "nodeA": [{"_tag": "a1","value": "a1"}],
	  "nodeB": [{"_priority": 100, "_tag": "b1","value": "b1"}],
	  "nodeC": [
		{"_tag":"c1","aTag":["a1"],"bTag":"b1"}
	  ]
	}
`
	json2 := `
	{
	  "log": {"level": "error"},
	  "nodeA": [{"_tag": "a2","value": "a2"}],
	  "nodeB": [{"_priority": -100, "_tag": "b2","value": "b2"}],
	  "nodeC": [
		{"aTag":["a2"],"bTag":"b2"},
		{"_tag":"c1","aTag":["a1.1"],"bTag":"b1.1"}
	  ]
	}
`
	expected := `
	{
	  // level is overwritten
	  "log": {"level": "error"},
	  "nodeA": [{"value": "a1"}, {"value": "a2"}],
	  "nodeB": [
		{"value": "b2"}, // the order is affected by priority
		{"value": "b1"}
	  ],
	  "nodeC": [
		// 3 items are merged into 2, and bTag is overwritten,
		// because 2 of them has same tag
		{"aTag":["a1","a1.1"],"bTag":"b1.1"},
		{"aTag":["a2"],"bTag":"b2"}
	  ]
	}
	`
	m, err := jsonsToMap(json1, json2)
	if err != nil {
		t.Error(err)
	}
	assertResult(t, m, expected)
}

func TestMergeTagValueTypes(t *testing.T) {
	json1 := `
	{
	  	"array_1": [{
			"_tag":"1",
			"array_2": [{
				"_tag":"2",
				"array_3.1": ["string",true,false],
				"array_3.2": [1,2,3],
				"number_1": 1,
				"number_2": 1,
				"bool_1": true,
				"bool_2": true
			}]
		}]
	}
`
	json2 := `
	{
		"array_1": [{
			"_tag":"1",
			"array_2": [{
				"_tag":"2",
				"array_3.1": [0,1,null],
				"array_3.2": null,
				"number_1": 0,
				"number_2": 1,
				"bool_1": true,
				"bool_2": false,
				"null_1": null
			}]
		}]
	}
`
	expected := `
	{
	  "array_1": [{
		"array_2": [{
			"array_3.1": ["string",true,false,0,1,null],
			"array_3.2": [1,2,3],
			"number_1": 0,
			"number_2": 1,
			"bool_1": true,
			"bool_2": false,
			"null_1": null
		}]
	  }]
	}
	`
	m, err := jsonsToMap(json1, json2)
	if err != nil {
		t.Error(err)
	}
	assertResult(t, m, expected)
}

func TestMergeTagDeep(t *testing.T) {
	json1 := `
	{
	  	"array_1": [{
			"_tag":"1",
			"array_2": [{
				"_tag":"2",
				"array_3": [true,false,"string"]
			}]
		}]
	}
`
	json2 := `
	{
	  	"array_1": [{
			"_tag":"1",
			"array_2": [{
				"_tag":"2",
				"_priority":-100,
				"array_3": [0,1,null]
			}]
		}]
	}
`
	expected := `
	{
	  	"array_1": [{
			"array_2": [{
				"array_3": [0,1,null,true,false,"string"]
			}]
		}]
	}
	`
	m, err := jsonsToMap(json1, json2)
	if err != nil {
		t.Error(err)
	}
	assertResult(t, m, expected)
}

func jsonsToMap(jsonStrs ...string) (map[string]interface{}, error) {
	merged := make(map[string]interface{})
	maps := make([]map[string]interface{}, len(jsonStrs))
	for _, j := range jsonStrs {
		m := map[string]interface{}{}
		json.Unmarshal([]byte(j), &m)
		maps = append(maps, m)
	}
	err := merge.Maps(merged, maps...)
	if err != nil {
		return nil, err
	}
	err = merge.ApplyRules(merged)
	if err != nil {
		return nil, err
	}
	merge.RemoveHelperFields(merged)
	return merged, nil
}

func assertResult(t *testing.T, got map[string]interface{}, want string) {
	e := make(map[string]interface{})
	err := jsonc.Decode(strings.NewReader(want), &e)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(got, e) {
		b, _ := json.Marshal(got)
		t.Fatalf("want:\n%s\n\ngot:\n%s", want, string(b))
	}
}
