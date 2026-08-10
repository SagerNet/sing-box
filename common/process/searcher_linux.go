//go:build linux

package process

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/contrab/freelru"
	"github.com/sagernet/sing/contrab/maphash"
)

const pathProc = "/proc"

var _ Searcher = (*linuxSearcher)(nil)

type linuxSearcher struct {
	logger           log.ContextLogger
	packageManager   tun.PackageManager
	diagConns        [4]*socketDiagConn
	processPathCache *freelru.Cache[uint32, *uidProcessPaths]
}

type uidProcessPaths struct {
	entries map[uint32]string
}

func NewSearcher(config Config) (Searcher, error) {
	processPathCache := common.Must1(freelru.New[uint32, *uidProcessPaths](64, maphash.NewHasher[uint32]().Hash32, true))
	processPathCache.SetLifetime(time.Second)
	searcher := &linuxSearcher{
		logger:           config.Logger,
		packageManager:   config.PackageManager,
		processPathCache: processPathCache,
	}
	for _, family := range []uint8{syscall.AF_INET, syscall.AF_INET6} {
		for _, protocol := range []uint8{syscall.IPPROTO_TCP, syscall.IPPROTO_UDP} {
			searcher.diagConns[socketDiagConnIndex(family, protocol)] = &socketDiagConn{
				family:   family,
				protocol: protocol,
				fd:       -1,
			}
		}
	}
	return searcher, nil
}

func (s *linuxSearcher) ResetCache() {
	s.processPathCache.Purge()
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
	processPath, err := s.findProcessPath(inode, uid)
	if err != nil {
		s.logger.DebugContext(ctx, "find process path: ", err)
	} else {
		processInfo.ProcessPath = processPath
	}
	if s.packageManager != nil {
		appID := uid % 100000
		var packageNames []string
		if sharedPackage, loaded := s.packageManager.SharedPackageByID(appID); loaded {
			packageNames = append(packageNames, sharedPackage)
		}
		if packages, loaded := s.packageManager.PackagesByID(appID); loaded {
			packageNames = append(packageNames, packages...)
		}
		processInfo.AndroidPackageNames = common.Uniq(packageNames)
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

func (s *linuxSearcher) findProcessPath(targetInode, uid uint32) (string, error) {
	if cached, ok := s.processPathCache.Get(uid); ok {
		if processPath, found := cached.entries[targetInode]; found {
			return processPath, nil
		}
	}
	processPaths, err := buildProcessPathsByUID(uid)
	if err != nil {
		return "", err
	}
	s.processPathCache.Add(uid, &uidProcessPaths{entries: processPaths})
	processPath, found := processPaths[targetInode]
	if !found {
		return "", E.New("process of uid(", uid, "), inode(", targetInode, ") not found")
	}
	return processPath, nil
}

func buildProcessPathsByUID(uid uint32) (map[uint32]string, error) {
	files, err := os.ReadDir(pathProc)
	if err != nil {
		return nil, err
	}
	buffer := make([]byte, syscall.PathMax)
	processPaths := make(map[uint32]string)
	for _, file := range files {
		if !file.IsDir() || !isPid(file.Name()) {
			continue
		}
		info, err := file.Info()
		if err != nil {
			if isIgnorableProcError(err) {
				continue
			}
			return nil, err
		}
		if info.Sys().(*syscall.Stat_t).Uid != uid {
			continue
		}
		processPath := filepath.Join(pathProc, file.Name())
		fdPath := filepath.Join(processPath, "fd")
		exePath, err := os.Readlink(filepath.Join(processPath, "exe"))
		if err != nil {
			if isIgnorableProcError(err) {
				continue
			}
			return nil, err
		}
		fds, err := os.ReadDir(fdPath)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			n, err := syscall.Readlink(filepath.Join(fdPath, fd.Name()), buffer)
			if err != nil {
				continue
			}
			inode, ok := parseSocketInode(buffer[:n])
			if !ok {
				continue
			}
			if _, loaded := processPaths[inode]; !loaded {
				processPaths[inode] = exePath
			}
		}
	}
	return processPaths, nil
}

func isIgnorableProcError(err error) bool {
	return os.IsNotExist(err) || os.IsPermission(err)
}

func parseSocketInode(link []byte) (uint32, bool) {
	const socketPrefix = "socket:["
	if len(link) <= len(socketPrefix) || string(link[:len(socketPrefix)]) != socketPrefix || link[len(link)-1] != ']' {
		return 0, false
	}
	var inode uint64
	for _, char := range link[len(socketPrefix) : len(link)-1] {
		if char < '0' || char > '9' {
			return 0, false
		}
		inode = inode*10 + uint64(char-'0')
		if inode > uint64(^uint32(0)) {
			return 0, false
		}
	}
	return uint32(inode), true
}

func isPid(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return !unicode.IsDigit(r)
	}) == -1
}
