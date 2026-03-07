//go:build with_gvisor && !android

package tailscale

import "github.com/sagernet/sing-box/adapter"

func setAndroidProtectFunc(platformInterface adapter.PlatformInterface) {
}
