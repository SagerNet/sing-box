package util

import (
	"strings"
)

type StringMap map[string]string

func (s StringMap) ToBytes() []byte {
	var lines []string
	for k, v := range s {
		lines = append(lines, k+"="+v)
	}
	return []byte(strings.Join(lines, "\n"))
}

func StringMapFromBytes(b []byte) StringMap {
	var m = make(StringMap)
	var lines = strings.Split(string(b), "\n")
	for _, line := range lines {
		v := strings.SplitN(line, "=", 2)
		if len(v) == 2 {
			m[v[0]] = v[1]
		}
	}
	return m
}
