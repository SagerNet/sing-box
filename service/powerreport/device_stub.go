//go:build !darwin || !cgo

package powerreport

func readDeviceState() (deviceState, bool) {
	return deviceState{}, false
}
