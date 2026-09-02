//go:build !badlinkname

package runtimeinfo

func collectGoroutines() *GoroutineReport {
	return nil
}
