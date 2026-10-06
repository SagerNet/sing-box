package kernelports

import "golang.org/x/sys/unix"

const (
	defaultEphemeralFirst = 49152
	defaultEphemeralLast  = 65535
)

func ephemeralRange() (uint16, uint16) {
	first, firstErr := unix.SysctlUint32("net.inet.ip.portrange.first")
	last, lastErr := unix.SysctlUint32("net.inet.ip.portrange.last")
	if firstErr != nil || lastErr != nil || first > last || last > 65535 {
		return defaultEphemeralFirst, defaultEphemeralLast
	}
	return uint16(first), uint16(last)
}
