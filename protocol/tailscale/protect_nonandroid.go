//go:build !android

package tailscale

import "github.com/sagernet/sing-box/experimental/libbox/platform"

func setAndroidProtectFunc(platformInterface platform.Interface) {
}
