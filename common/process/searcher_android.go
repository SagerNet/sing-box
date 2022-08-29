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
	_, uid, err := resolveSocketByNetlink(network, source, destination)
	if err != nil {
		return nil, err
	}
	if sharedPackage, loaded := s.packageManager.SharedPackageByID(uid % 100000); loaded {
		return &Info{
			UserId:      int32(uid),
			PackageName: sharedPackage,
		}, nil
	}
	if packageName, loaded := s.packageManager.PackageByID(uid % 100000); loaded {
		return &Info{
			UserId:      int32(uid),
			PackageName: packageName,
		}, nil
	}
	return &Info{UserId: int32(uid)}, nil
}
