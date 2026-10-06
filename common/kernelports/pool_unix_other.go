//go:build !windows && !linux && !darwin

package kernelports

func ephemeralRange() (uint16, uint16) {
	return 49152, 65535
}
