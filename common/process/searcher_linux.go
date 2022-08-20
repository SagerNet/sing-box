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

func (s *linuxSearcher) FindProcessInfo(ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*Info, error) {
	socket, err := resolveSocketByNetlink(network, source, destination)
	if err != nil {
		return nil, err
	}
	processPath, err := resolveProcessNameByProcSearch(socket.INode, socket.UID)
	if err != nil {
		s.logger.DebugContext(ctx, "find process path: ", err)
	}
	return &Info{
		UserId:      int32(socket.UID),
		ProcessPath: processPath,
	}, nil
}
