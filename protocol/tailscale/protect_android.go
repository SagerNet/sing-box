package tailscale

import (
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/tailscale/net/netns"
)

func setAndroidProtectFunc(platformInterface platform.Interface) {
	if platformInterface != nil {
		netns.SetAndroidProtectFunc(func(fd int) error {
			return platformInterface.AutoDetectInterfaceControl(fd)
		})
	} else {
		netns.SetAndroidProtectFunc(nil)
	}
}
