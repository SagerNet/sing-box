package baderror

import "strings"

func Contains(err error, msgList ...string) bool {
	for _, msg := range msgList {
		if strings.Contains(err.Error(), msg) {
			return true
		}
	}
	return false
}
