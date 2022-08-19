package hysteria

import (
	"regexp"
	"strconv"
)

func StringToBps(s string) uint64 {
	if s == "" {
		return 0
	}
	m := regexp.MustCompile(`^(\d+)\s*([KMGT]?)([Bb])ps$`).FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	var n uint64
	switch m[2] {
	case "K":
		n = 1 << 10
	case "M":
		n = 1 << 20
	case "G":
		n = 1 << 30
	case "T":
		n = 1 << 40
	default:
		n = 1
	}
	v, _ := strconv.ParseUint(m[1], 10, 64)
	n = v * n
	if m[3] == "b" {
		// Bits, need to convert to bytes
		n = n >> 3
	}
	return n
}
