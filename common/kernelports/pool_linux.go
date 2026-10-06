package kernelports

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultEphemeralFirst = 32768
	defaultEphemeralLast  = 60999
)

func ephemeralRange() (uint16, uint16) {
	content, err := os.ReadFile("/proc/sys/net/ipv4/ip_local_port_range")
	if err != nil {
		return defaultEphemeralFirst, defaultEphemeralLast
	}
	fields := strings.Fields(string(content))
	if len(fields) != 2 {
		return defaultEphemeralFirst, defaultEphemeralLast
	}
	first, firstErr := strconv.ParseUint(fields[0], 10, 16)
	last, lastErr := strconv.ParseUint(fields[1], 10, 16)
	if firstErr != nil || lastErr != nil || first > last {
		return defaultEphemeralFirst, defaultEphemeralLast
	}
	return uint16(first), uint16(last)
}
