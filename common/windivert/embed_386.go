//go:build windows && 386 && !with_external_windivert

package windivert

import _ "embed"

//go:embed assets/WinDivert32.sys
var sysBytes []byte
