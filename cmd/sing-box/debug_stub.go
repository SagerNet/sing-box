//go:build debug && !linux

package main

func rusageMaxRSS() float64 {
	return -1
}
