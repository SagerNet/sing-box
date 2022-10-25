//go:build with_gvisor && linux

package libbox

import (
	"github.com/sagernet/sing-tun"

	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

var _ tun.GVisorTun = (*nativeTun)(nil)

func (t *nativeTun) NewEndpoint() (stack.LinkEndpoint, error) {
	return fdbased.New(&fdbased.Options{
		FDs: []int{t.tunFd},
		MTU: t.tunMTU,
	})
}
