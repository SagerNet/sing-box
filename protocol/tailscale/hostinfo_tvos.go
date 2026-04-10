//go:build with_gvisor && tvos

package tailscale

import (
	_ "unsafe"

	"github.com/sagernet/tailscale/types/lazy"
)

//go:linkname isAppleTV github.com/sagernet/tailscale/version.isAppleTV
var isAppleTV lazy.SyncValue[bool]

func init() {
	isAppleTV.Set(true)
}
