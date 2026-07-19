//go:build linux && !android

package process

import (
	"context"
	"errors"
	"net/netip"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ Searcher = (*linuxSearcher)(nil)

type linuxSearcher struct {
	logger           log.ContextLogger
	diagConns        [4]*socketDiagConn
	processPathCache *uidProcessPathCache
}

func NewSearcher(config Config) (Searcher, error) {
	searcher := &linuxSearcher{
		logger:           config.Logger,
		processPathCache: newUIDProcessPathCache(time.Second),
	}
	for _, family := range []uint8{syscall.AF_INET, syscall.AF_INET6} {
		for _, protocol := range []uint8{syscall.IPPROTO_TCP, syscall.IPPROTO_UDP} {
			searcher.diagConns[socketDiagConnIndex(family, protocol)] = newSocketDiagConn(family, protocol)
		}
	}
	return searcher, nil
}

func (s *linuxSearcher) ResetCache() {
	s.processPathCache.cache.Purge()
}

func (s *linuxSearcher) Close() error {
	var errs []error
	for _, conn := range s.diagConns {
		if conn == nil {
			continue
		}
		errs = append(errs, conn.Close())
	}
	return E.Errors(errs...)
}

func (s *linuxSearcher) FindProcessInfo(ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*adapter.ConnectionOwner, error) {
	inode, uid, err := s.resolveSocketByNetlink(network, source, destination)
	if err != nil {
		return nil, err
	}
	processInfo := &adapter.ConnectionOwner{
		UserId: int32(uid),
	}
	processPath, err := s.processPathCache.findProcessPath(inode, uid)
	if err != nil {
		s.logger.DebugContext(ctx, "find process path: ", err)
	} else {
		processInfo.ProcessPath = processPath
	}
	return processInfo, nil
}

func (s *linuxSearcher) resolveSocketByNetlink(network string, source netip.AddrPort, destination netip.AddrPort) (inode, uid uint32, err error) {
	family, protocol, err := socketDiagSettings(network, source)
	if err != nil {
		return 0, 0, err
	}
	conn := s.diagConns[socketDiagConnIndex(family, protocol)]
	if conn == nil {
		return 0, 0, E.New("missing socket diag connection for family=", family, " protocol=", protocol)
	}
	if destination.IsValid() && source.Addr().BitLen() == destination.Addr().BitLen() {
		inode, uid, err = conn.query(source, destination)
		if err == nil {
			return inode, uid, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return 0, 0, err
		}
	}
	return querySocketDiagOnce(family, protocol, source)
}
