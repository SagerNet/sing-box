package libbox

import (
	"os"
	_ "unsafe"
)

// https://github.com/SagerNet/sing-box/issues/3233
// https://github.com/golang/go/issues/70508
// https://github.com/tailscale/tailscale/issues/13452

//go:linkname checkPidfdOnce os.checkPidfdOnce
var checkPidfdOnce func() error

func init() {
	checkPidfdOnce = func() error {
		return os.ErrInvalid
	}
}
