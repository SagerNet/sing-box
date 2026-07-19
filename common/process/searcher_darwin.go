//go:build darwin

package process

import (
	"context"
	"net/netip"
	"strconv"
	"strings"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
)

var _ Searcher = (*darwinSearcher)(nil)

type darwinSearcher struct{}

func NewSearcher(_ Config) (Searcher, error) {
	return &darwinSearcher{}, nil
}

func (d *darwinSearcher) ResetCache() {
	sharedDarwinConnectionFinder.resetCache()
}

func (d *darwinSearcher) Close() error {
	return nil
}

func (d *darwinSearcher) FindProcessInfo(ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*adapter.ConnectionOwner, error) {
	return FindDarwinConnectionOwner(network, source, destination)
}

var structSize = func() int {
	value, _ := syscall.Sysctl("kern.osrelease")
	major, _, _ := strings.Cut(value, ".")
	n, _ := strconv.ParseInt(major, 10, 64)
	switch true {
	case n >= 22:
		return 408
	default:
		// from darwin-xnu/bsd/netinet/in_pcblist.c:get_pcblist_n
		// size/offset are round up (aligned) to 8 bytes in darwin
		// rup8(sizeof(xinpcb_n)) + rup8(sizeof(xsocket_n)) +
		// 2 * rup8(sizeof(xsockbuf_n)) + rup8(sizeof(xsockstat_n))
		return 384
	}
}()
