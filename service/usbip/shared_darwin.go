//go:build darwin && cgo

package usbip

func hex8(v uint8) string {
	const hexdigits = "0123456789abcdef"
	return string([]byte{hexdigits[(v>>4)&0xf], hexdigits[v&0xf]})
}
