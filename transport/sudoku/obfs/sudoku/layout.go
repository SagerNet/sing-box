package sudoku

import (
	"fmt"
	"math/bits"
	"sort"
	"strings"
)

type byteLayout struct {
	name        string
	hintMask    byte
	hintValue   byte
	padMarker   byte
	paddingPool []byte

	encodeHint  func(val, pos byte) byte
	encodeGroup func(group byte) byte
	decodeGroup func(b byte) (byte, bool)
}

func (l *byteLayout) isHint(b byte) bool {
	return (b & l.hintMask) == l.hintValue
}

// resolveLayout picks the byte layout based on ASCII preference and optional custom pattern.
// ASCII always wins if requested. Custom patterns are ignored when ASCII is preferred.
func resolveLayout(mode string, customPattern string) (*byteLayout, error) {
	switch strings.ToLower(mode) {
	case "ascii", "prefer_ascii":
		return newASCIILayout(), nil
	case "entropy", "prefer_entropy", "":
		// fall back to entropy unless a custom pattern is provided
	default:
		return nil, fmt.Errorf("invalid ascii mode: %s", mode)
	}

	if strings.TrimSpace(customPattern) != "" {
		return newCustomLayout(customPattern)
	}
	return newEntropyLayout(), nil
}

func newASCIILayout() *byteLayout {
	padding := make([]byte, 0, 32)
	for i := 0; i < 32; i++ {
		padding = append(padding, byte(0x20+i))
	}
	return &byteLayout{
		name:        "ascii",
		hintMask:    0x40,
		hintValue:   0x40,
		padMarker:   0x3F,
		paddingPool: padding,
		encodeHint: func(val, pos byte) byte {
			return 0x40 | ((val & 0x03) << 4) | (pos & 0x0F)
		},
		encodeGroup: func(group byte) byte {
			return 0x40 | (group & 0x3F)
		},
		decodeGroup: func(b byte) (byte, bool) {
			if (b & 0x40) == 0 {
				return 0, false
			}
			return b & 0x3F, true
		},
	}
}

func newEntropyLayout() *byteLayout {
	padding := make([]byte, 0, 16)
	for i := 0; i < 8; i++ {
		padding = append(padding, byte(0x80+i))
		padding = append(padding, byte(0x10+i))
	}
	return &byteLayout{
		name:        "entropy",
		hintMask:    0x90,
		hintValue:   0x00,
		padMarker:   0x80,
		paddingPool: padding,
		encodeHint: func(val, pos byte) byte {
			return ((val & 0x03) << 5) | (pos & 0x0F)
		},
		encodeGroup: func(group byte) byte {
			v := group & 0x3F
			return ((v & 0x30) << 1) | (v & 0x0F)
		},
		decodeGroup: func(b byte) (byte, bool) {
			if (b & 0x90) != 0 {
				return 0, false
			}
			return ((b >> 1) & 0x30) | (b & 0x0F), true
		},
	}
}

func newCustomLayout(pattern string) (*byteLayout, error) {
	cleaned := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(pattern), " ", ""))
	if len(cleaned) != 8 {
		return nil, fmt.Errorf("custom table must have 8 symbols, got %d", len(cleaned))
	}

	var xBits, pBits, vBits []uint8
	for i, c := range cleaned {
		bit := uint8(7 - i)
		switch c {
		case 'x':
			xBits = append(xBits, bit)
		case 'p':
			pBits = append(pBits, bit)
		case 'v':
			vBits = append(vBits, bit)
		default:
			return nil, fmt.Errorf("invalid char %q in custom table", c)
		}
	}

	if len(xBits) != 2 || len(pBits) != 2 || len(vBits) != 4 {
		return nil, fmt.Errorf("custom table must contain exactly 2 x, 2 p, 4 v")
	}

	xMask := byte(0)
	for _, b := range xBits {
		xMask |= 1 << b
	}

	encodeBits := func(val, pos byte, dropX int) byte {
		var out byte
		out |= xMask
		if dropX >= 0 {
			out &^= 1 << xBits[dropX]
		}
		if (val & 0x02) != 0 {
			out |= 1 << pBits[0]
		}
		if (val & 0x01) != 0 {
			out |= 1 << pBits[1]
		}
		for i, bit := range vBits {
			if (pos>>(3-uint8(i)))&0x01 == 1 {
				out |= 1 << bit
			}
		}
		return out
	}

	decodeGroup := func(b byte) (byte, bool) {
		if (b & xMask) != xMask {
			return 0, false
		}
		var val, pos byte
		if b&(1<<pBits[0]) != 0 {
			val |= 0x02
		}
		if b&(1<<pBits[1]) != 0 {
			val |= 0x01
		}
		for i, bit := range vBits {
			if b&(1<<bit) != 0 {
				pos |= 1 << (3 - uint8(i))
			}
		}
		group := (val << 4) | (pos & 0x0F)
		return group, true
	}

	paddingSet := make(map[byte]struct{})
	var padding []byte
	for drop := range xBits {
		for val := 0; val < 4; val++ {
			for pos := 0; pos < 16; pos++ {
				b := encodeBits(byte(val), byte(pos), drop)
				if bits.OnesCount8(b) >= 5 {
					if _, ok := paddingSet[b]; !ok {
						paddingSet[b] = struct{}{}
						padding = append(padding, b)
					}
				}
			}
		}
	}
	sort.Slice(padding, func(i, j int) bool { return padding[i] < padding[j] })
	if len(padding) == 0 {
		return nil, fmt.Errorf("custom table produced empty padding pool")
	}

	return &byteLayout{
		name:        fmt.Sprintf("custom(%s)", cleaned),
		hintMask:    xMask,
		hintValue:   xMask,
		padMarker:   padding[0],
		paddingPool: padding,
		encodeHint: func(val, pos byte) byte {
			return encodeBits(val, pos, -1)
		},
		encodeGroup: func(group byte) byte {
			val := (group >> 4) & 0x03
			pos := group & 0x0F
			return encodeBits(val, pos, -1)
		},
		decodeGroup: decodeGroup,
	}, nil
}

