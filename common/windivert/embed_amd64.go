//go:build windows && amd64 && !with_external_windivert

package windivert

import _ "embed"

//go:embed assets/WinDivert64.sys
var sysBytes []byte
