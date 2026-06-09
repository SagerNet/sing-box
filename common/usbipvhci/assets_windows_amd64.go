//go:build windows && amd64

package usbipvhci

import _ "embed"

//go:embed assets/amd64/usbip2_ude.sys
var udeSys []byte

//go:embed assets/amd64/usbip2_ude.inf
var udeInf []byte

//go:embed assets/amd64/usbip2_ude.cat
var udeCat []byte

//go:embed assets/amd64/usbip2_filter.sys
var filterSys []byte

//go:embed assets/amd64/usbip2_filter.inf
var filterInf []byte

//go:embed assets/amd64/usbip2_filter.cat
var filterCat []byte

// assetVersion is the bundled usbip-win2 release. amd64 uses 0.9.7.7
// (latest; x64-only installer), the first release with PLUGIN_HARDWARE_ONCE.
func assetVersion() string { return "0.9.7.7" }

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
