//go:build darwin && !cgo

package powerreport

func readSystemUsage() systemUsage {
	panic("power report requires CGO on darwin, rebuild with CGO_ENABLED=1")
}

func readClocks() (absoluteTime int64, continuousTime int64) {
	panic("power report requires CGO on darwin, rebuild with CGO_ENABLED=1")
}

func readInterfaceCounters() map[string]interfaceCounters {
	panic("power report requires CGO on darwin, rebuild with CGO_ENABLED=1")
}
