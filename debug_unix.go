//go:build linux || darwin

package box

import (
	"runtime"
	"syscall"
)

func rusageMaxRSS() float64 {
	ru := syscall.Rusage{}
	err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	if err != nil {
		return 0
	}

	rss := float64(ru.Maxrss)
	if runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
		rss /= 1 << 20 // ru_maxrss is bytes on darwin
	} else {
		// ru_maxrss is kilobytes elsewhere (linux, openbsd, etc)
		rss /= 1 << 10
	}
	return rss
}
