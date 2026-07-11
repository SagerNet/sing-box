//go:build windows && amd64

package windivert

import _ "embed"

//go:embed assets/WinDivert64.sys
var sysBytes []byte

func driverSysName() string { return "WinDivert64.sys" }
