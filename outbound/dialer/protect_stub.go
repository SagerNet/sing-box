//go:build !android && !with_protect

package dialer

import "github.com/sagernet/sing/common/control"

func ProtectPath(protectPath string) control.Func {
	return nil
}
