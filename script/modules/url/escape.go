package url

import "strings"

var tblEscapeURLQuery = [128]byte{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 1, 0, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0,
}

// The code below is mostly borrowed from the standard Go url package

const upperhex = "0123456789ABCDEF"

func escape(s string, table *[128]byte, spaceToPlus bool) string {
	spaceCount, hexCount := 0, 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c > 127 || table[c] == 0 {
			if c == ' ' && spaceToPlus {
				spaceCount++
			} else {
				hexCount++
			}
		}
	}

	if spaceCount == 0 && hexCount == 0 {
		return s
	}

	var sb strings.Builder
	hexBuf := [3]byte{'%', 0, 0}

	sb.Grow(len(s) + 2*hexCount)

	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == ' ' && spaceToPlus:
			sb.WriteByte('+')
		case c > 127 || table[c] == 0:
			hexBuf[1] = upperhex[c>>4]
			hexBuf[2] = upperhex[c&15]
			sb.Write(hexBuf[:])
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}
