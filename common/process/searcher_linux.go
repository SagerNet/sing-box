//go:build linux && !android

package process

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing-box/log"
)

var _ Searcher = (*linuxSearcher)(nil)

type linuxSearcher struct {
	logger log.ContextLogger
}

func NewSearcher(config Config) (Searcher, error) {
	return &linuxSearcher{config.Logger}, nil
}

func (s *linuxSearcher) FindProcessInfo(ctx context.Context, network string, srcIP netip.Addr, srcPort int) (*Info, error) {
	inode, uid, err := resolveSocketByNetlink(network, srcIP, srcPort)
	if err != nil {
		return nil, err
	}
	processPath, err := resolveProcessNameByProcSearch(inode, uid)
	if err != nil {
		s.logger.DebugContext(ctx, "find process path: ", err)
	}
	return &Info{
		UserId:      uid,
		ProcessPath: processPath,
	}, nil
}
