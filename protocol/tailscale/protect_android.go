package tailscale

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/tailscale/net/netns"
)

func setAndroidProtectFunc(platformInterface adapter.PlatformInterface) {
	if platformInterface != nil {
		netns.SetAndroidProtectFunc(func(fd int) error {
			return platformInterface.AutoDetectInterfaceControl(fd)
		})
	} else {
		netns.SetAndroidProtectFunc(nil)
	}
}
