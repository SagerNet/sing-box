//go:build !(linux || darwin)

package box

func rusageMaxRSS() float64 {
	return -1
}
