package jsonc_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/sagernet/sing-box/common/conf/jsonc"
)

func TestReader(t *testing.T) {
	data := []struct {
		input  string
		output string
	}{
		{
			`
content #comment 1
#comment 2
content 2`,
			`
content 

content 2`},
		{`content`, `content`},
		{" ", " "},
		{`con/*abcd*/tent`, "content"},
		{`
text // adlkhdf /*
//comment adfkj
text 2*/`, `
text 

text 2*`},
		{`"//"content`, `"//"content`},
		{`abcd'//'abcd`, `abcd'//'abcd`},
		{`"\""`, `"\""`},
		{`\"/*abcd*/\"`, `\"\"`},
	}

	for _, testCase := range data {
		reader := &jsonc.Reader{
			Reader: bytes.NewReader([]byte(testCase.input)),
		}

		buf := make([]byte, 1024)
		n, err := reader.Read(buf)
		if err != nil {
			t.Error(err)
			return
		}
		got := buf[:n]
		if string(got) != testCase.output {
			t.Errorf("want: %s, got: %s", testCase.output, got)
			return
		}
	}
}

func TestReader1(t *testing.T) {
	type dataStruct struct {
		input  string
		output string
	}

	bufLen := 8

	data := []dataStruct{
		{"loooooooooooooooooooooooooooooooooooooooog", "loooooooooooooooooooooooooooooooooooooooog"},
		{`{"t": "\/testlooooooooooooooooooooooooooooong"}`, `{"t": "\/testlooooooooooooooooooooooooooooong"}`},
		{`{"t": "\/test"}`, `{"t": "\/test"}`},
		{`"\// fake comment"`, `"\// fake comment"`},
		{`"\/\/\/\/\/"`, `"\/\/\/\/\/"`},
	}

	for _, testCase := range data {
		reader := &jsonc.Reader{
			Reader: bytes.NewReader([]byte(testCase.input)),
		}
		target := make([]byte, 0)
		buf := make([]byte, bufLen)
		var n int
		var err error
		for n, err = reader.Read(buf); err == nil; n, err = reader.Read(buf) {
			if n > len(buf) {
				t.Error("n: ", n)
			}
			target = append(target, buf[:n]...)
			buf = make([]byte, bufLen)
		}
		if err != nil && err != io.EOF {
			t.Error("error: ", err)
		}
		got := string(target)
		if string(got) != testCase.output {
			t.Errorf("want: %s, got: %s", testCase.output, got)
			return
		}
	}
}
