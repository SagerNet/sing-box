//go:build !linux

package box

func rusageMaxRSS() float64 {
	return -1
}
