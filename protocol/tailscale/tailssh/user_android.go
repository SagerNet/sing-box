//go:build with_gvisor && android

package tailssh

import (
	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
)

func resolveLocalUserNative(username string) (*adapter.PlatformUser, error) {
	return nil, E.New("native user resolution not supported on android")
}

func defaultShell() string {
	return "/system/bin/sh"
}
