package process

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ Searcher = (*androidSearcher)(nil)

type androidSearcher struct {
	packageManager tun.PackageManager
}

func NewSearcher(config Config) (Searcher, error) {
	if config.PackageManager == nil {
		return nil, E.New("missing package manager")
	}
	return &androidSearcher{config.PackageManager}, nil
}

func (s *androidSearcher) Close() error {
	return nil
}

func (s *androidSearcher) FindProcessInfo(ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*adapter.ConnectionOwner, error) {
	family, protocol, err := socketDiagSettings(network, source)
	if err != nil {
		return nil, err
	}
	_, uid, err := querySocketDiagOnce(family, protocol, source)
	if err != nil {
		return nil, err
	}
	appID := uid % 100000
	var packageNames []string
	if sharedPackage, loaded := s.packageManager.SharedPackageByID(appID); loaded {
		packageNames = append(packageNames, sharedPackage)
	}
	if packages, loaded := s.packageManager.PackagesByID(appID); loaded {
		packageNames = append(packageNames, packages...)
	}
	packageNames = common.Uniq(packageNames)
	return &adapter.ConnectionOwner{
		UserId:              int32(uid),
		AndroidPackageNames: packageNames,
	}, nil
}
