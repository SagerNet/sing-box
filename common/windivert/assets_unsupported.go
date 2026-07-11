//go:build windows && !amd64 && !386

package windivert

var sysBytes []byte

func driverSysName() string { return "" }
