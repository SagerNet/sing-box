//go:build windows && amd64

package vboxusb

import _ "embed"

//go:embed assets/amd64/VBoxUSB.sys
var vboxUSBSys []byte

//go:embed assets/amd64/VBoxUSB.inf
var vboxUSBInf []byte

//go:embed assets/amd64/VBoxUSB.cat
var vboxUSBCat []byte

//go:embed assets/amd64/VBoxUSBMon.sys
var vboxUSBMonSys []byte

func assetFiles() []assetFile {
	return []assetFile{
		{"VBoxUSB.sys", vboxUSBSys},
		{"VBoxUSB.inf", vboxUSBInf},
		{"VBoxUSB.cat", vboxUSBCat},
		{"VBoxUSBMon.sys", vboxUSBMonSys},
	}
}
