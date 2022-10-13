package jsonc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type offset struct {
	line int
	char int
}

func findOffset(b []byte, o int) *offset {
	if o >= len(b) || o < 0 {
		return nil
	}
	line := 1
	char := 0
	for i, x := range b {
		if i == o {
			break
		}
		if x == '\n' {
			line++
			char = 0
		} else {
			char++
		}
	}
	return &offset{line: line, char: char}
}

// Decode reads from reader and decode into target
// syntax error could be detected.
func Decode(reader io.Reader, target interface{}) error {
	jsonContent := bytes.NewBuffer(make([]byte, 0, 10240))
	jsonReader := io.TeeReader(&Reader{
		Reader: reader,
	}, jsonContent)
	decoder := json.NewDecoder(jsonReader)

	if err := decoder.Decode(target); err != nil {
		var pos *offset
		if tErr, ok := err.(*json.SyntaxError); ok {
			pos = findOffset(jsonContent.Bytes(), int(tErr.Offset))
		} else if tErr, ok := err.(*json.UnmarshalTypeError); ok {
			pos = findOffset(jsonContent.Bytes(), int(tErr.Offset))
		}
		if pos != nil {
			return fmt.Errorf("failed to decode jsonc at line %d char %d: %s", pos.line, pos.char, err)
		}
		return fmt.Errorf("failed to decode jsonc: %s", err)
	}
	return nil
}
