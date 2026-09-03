package powerreport

/*
#cgo LDFLAGS: -framework Foundation
#cgo ios LDFLAGS: -framework UIKit
#cgo !ios LDFLAGS: -framework CoreFoundation -framework IOKit

#include "device_darwin.h"
*/
import "C"

func readDeviceState() (deviceState, bool) {
	var batteryLevel C.int
	state := deviceState{
		LowPowerMode: C.boxPowerLowPowerMode() != 0,
		ThermalState: thermalStateName(int(C.boxPowerThermalState())),
	}
	switch C.boxPowerSource(&batteryLevel) {
	case 1:
		state.PowerSource = "battery"
	case 2:
		state.PowerSource = "ac"
	}
	state.BatteryLevel = int(batteryLevel)
	return state, true
}

func thermalStateName(state int) string {
	switch state {
	case 0:
		return "nominal"
	case 1:
		return "fair"
	case 2:
		return "serious"
	case 3:
		return "critical"
	default:
		return ""
	}
}
