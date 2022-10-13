package jsonc_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/common/conf/jsonc"
)

func TestLoaderError(t *testing.T) {
	testCases := []struct {
		Input  string
		Output string
	}{
		{
			Input: `{
				"log": {
					// abcd
					0,
					"loglevel": "info"
				}
		}`,
			Output: "line 4 char 6",
		},
		{
			Input: `{
				"log": {
					// abcd
					"loglevel": "info",
				}
		}`,
			Output: "line 5 char 5",
		},
	}
	for _, testCase := range testCases {
		reader := bytes.NewReader([]byte(testCase.Input))
		m := make(map[string]interface{})
		err := jsonc.Decode(reader, m)
		errString := err.Error()
		if !strings.Contains(errString, testCase.Output) {
			t.Error("unexpected output from json: ", testCase.Input, ". expected ", testCase.Output, ", but actually ", errString)
		}
	}
}
