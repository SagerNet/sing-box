//go:build with_gvisor

package xdp

import _ "embed"

//go:embed xdp_prog_amd64.o
var xdpProgData []byte
