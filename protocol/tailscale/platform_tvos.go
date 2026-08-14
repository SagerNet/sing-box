//go:build tvos

package tailscale

import "github.com/sagernet/tailscale/version"

func init() {
	version.SetAppleTV()
}
