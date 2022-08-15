package process

import (
	"context"
	"net/netip"

	tun "github.com/sagernet/sing-tun"
)

var _ Searcher = (*androidSearcher)(nil)

type androidSearcher struct {
	packageManager tun.PackageManager
}

func NewSearcher(config Config) (Searcher, error) {
	return &androidSearcher{config.PackageManager}, nil
}

func (s *androidSearcher) FindProcessInfo(ctx context.Context, network string, srcIP netip.Addr, srcPort int) (*Info, error) {
	_, uid, err := resolveSocketByNetlink(network, srcIP, srcPort)
	if err != nil {
		return nil, err
	}
	if sharedPackage, loaded := s.packageManager.SharedPackageByID(uint32(uid)); loaded {
		return &Info{
			UserId:      uid,
			PackageName: sharedPackage,
		}, nil
	}
	if packageName, loaded := s.packageManager.PackageByID(uint32(uid)); loaded {
		return &Info{
			UserId:      uid,
			PackageName: packageName,
		}, nil
	}
	return &Info{UserId: uid}, nil
}
