//go:build !unix

package libbox

import (
	"net"
)

func linkFlags(rawFlags uint32) net.Flags {
	panic("stub!")
}
