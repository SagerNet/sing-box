package process

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing-tun"
)

var _ Searcher = (*androidSearcher)(nil)

type androidSearcher struct {
	packageManager tun.PackageManager
}

func NewSearcher(config Config) (Searcher, error) {
	return &androidSearcher{config.PackageManager}, nil
}

func (s *androidSearcher) FindProcessInfo(ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*Info, error) {
	socket, err := resolveSocketByNetlink(network, source, destination)
	if err != nil {
		return nil, err
	}
	if sharedPackage, loaded := s.packageManager.SharedPackageByID(socket.UID); loaded {
		return &Info{
			UserId:      int32(socket.UID),
			PackageName: sharedPackage,
		}, nil
	}
	if packageName, loaded := s.packageManager.PackageByID(socket.UID); loaded {
		return &Info{
			UserId:      int32(socket.UID),
			PackageName: packageName,
		}, nil
	}
	return &Info{UserId: int32(socket.UID)}, nil
}
