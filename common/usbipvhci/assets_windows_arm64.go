//go:build windows && arm64

package usbipvhci

import _ "embed"

//go:embed assets/arm64/usbip2_ude.sys
var udeSys []byte

//go:embed assets/arm64/usbip2_ude.inf
var udeInf []byte

//go:embed assets/arm64/usbip2_ude.cat
var udeCat []byte

//go:embed assets/arm64/usbip2_filter.sys
var filterSys []byte

//go:embed assets/arm64/usbip2_filter.inf
var filterInf []byte

//go:embed assets/arm64/usbip2_filter.cat
var filterCat []byte

// assetVersion is the bundled usbip-win2 release. arm64 uses 0.9.7.5, the
// newest release with an arm64 installer (0.9.7.7 is x64-only); it shares
// the amd64 ABI, including PLUGIN_HARDWARE_ONCE.
func assetVersion() string { return "0.9.7.5" }

func assetFiles() []assetFile {
	return []assetFile{
		{"usbip2_ude.sys", udeSys},
		{"usbip2_ude.inf", udeInf},
		{"usbip2_ude.cat", udeCat},
		{"usbip2_filter.sys", filterSys},
		{"usbip2_filter.inf", filterInf},
		{"usbip2_filter.cat", filterCat},
	}
}
