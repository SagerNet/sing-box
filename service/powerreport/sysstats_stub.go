//go:build !darwin && !linux && !windows

package powerreport

func readSystemUsage() systemUsage {
	return systemUsage{}
}

func readClocks() (absoluteTime int64, continuousTime int64) {
	return 0, 0
}

func readInterfaceCounters() map[string]interfaceCounters {
	return nil
}
