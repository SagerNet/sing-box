package powerreport

import (
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func readSystemUsage() systemUsage {
	var rusage unix.Rusage
	err := unix.Getrusage(unix.RUSAGE_SELF, &rusage)
	if err != nil {
		return systemUsage{}
	}
	return systemUsage{
		valid:            true,
		userTime:         rusage.Utime.Nano(),
		systemTime:       rusage.Stime.Nano(),
		diskBytesWritten: readWriteBytes(),
	}
}

func readWriteBytes() uint64 {
	content, err := os.ReadFile("/proc/self/io")
	if err != nil {
		return 0
	}
	for line := range strings.SplitSeq(string(content), "\n") {
		value, found := strings.CutPrefix(line, "write_bytes: ")
		if !found {
			continue
		}
		parsed, parseErr := strconv.ParseUint(value, 10, 64)
		if parseErr != nil {
			return 0
		}
		return parsed
	}
	return 0
}

func readClocks() (absoluteTime int64, continuousTime int64) {
	var monotonicTime unix.Timespec
	err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &monotonicTime)
	if err != nil {
		return 0, 0
	}
	var bootTime unix.Timespec
	err = unix.ClockGettime(unix.CLOCK_BOOTTIME, &bootTime)
	if err != nil {
		return 0, 0
	}
	return monotonicTime.Nano(), bootTime.Nano()
}

func readInterfaceCounters() map[string]interfaceCounters {
	content, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) <= 2 {
		return nil
	}
	result := make(map[string]interfaceCounters)
	for _, line := range lines[2:] {
		name, counters, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "lo" || strings.HasPrefix(name, "tun") || strings.HasPrefix(name, "utun") || strings.HasPrefix(name, "dummy") {
			continue
		}
		fields := strings.Fields(counters)
		if len(fields) < 10 {
			continue
		}
		inPackets, inErr := strconv.ParseUint(fields[1], 10, 64)
		outPackets, outErr := strconv.ParseUint(fields[9], 10, 64)
		if inErr != nil || outErr != nil {
			continue
		}
		result[name] = interfaceCounters{
			inPackets:  uint32(inPackets),
			outPackets: uint32(outPackets),
		}
	}
	return result
}
