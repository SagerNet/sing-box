package certificate

import (
	"context"
	"crypto/x509"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sagernet/fswatch"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"
)

var _ adapter.CertificateStore = (*Store)(nil)

type Store struct {
	access                    sync.RWMutex
	systemPool                *x509.CertPool
	currentPool               *x509.CertPool
	certificate               string
	certificatePaths          []string
	certificateDirectoryPaths []string
	watcher                   *fswatch.Watcher
}

func NewStore(ctx context.Context, logger logger.Logger, options option.CertificateOptions) (*Store, error) {
	var systemPool *x509.CertPool
	switch options.Store {
	case C.CertificateStoreSystem, "":
		systemPool = x509.NewCertPool()
		platformInterface := service.FromContext[platform.Interface](ctx)
		var systemValid bool
		if platformInterface != nil {
			for _, cert := range platformInterface.SystemCertificates() {
				if systemPool.AppendCertsFromPEM([]byte(cert)) {
					systemValid = true
				}
			}
		}
		if !systemValid {
			certPool, err := x509.SystemCertPool()
			if err != nil {
				return nil, err
			}
			systemPool = certPool
		}
	case C.CertificateStoreMozilla:
		systemPool = mozillaIncluded
	case C.CertificateStoreNone:
		systemPool = nil
	default:
		return nil, E.New("unknown certificate store: ", options.Store)
	}
	store := &Store{
		systemPool:                systemPool,
		certificate:               strings.Join(options.Certificate, "\n"),
		certificatePaths:          options.CertificatePath,
		certificateDirectoryPaths: options.CertificateDirectoryPath,
	}
	var watchPaths []string
	for _, target := range options.CertificatePath {
		watchPaths = append(watchPaths, target)
	}
	for _, target := range options.CertificateDirectoryPath {
		watchPaths = append(watchPaths, target)
	}
	if len(watchPaths) > 0 {
		watcher, err := fswatch.NewWatcher(fswatch.Options{
			Path:   watchPaths,
			Logger: logger,
			Callback: func(_ string) {
				err := store.update()
				if err != nil {
					logger.Error(E.Cause(err, "reload certificates"))
				}
			},
		})
		if err != nil {
			return nil, E.Cause(err, "fswatch: create fsnotify watcher")
		}
		store.watcher = watcher
	}
	err := store.update()
	if err != nil {
		return nil, E.Cause(err, "initializing certificate store")
	}
	return store, nil
}

func (s *Store) Name() string {
	return "certificate"
}

func (s *Store) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if s.watcher != nil {
		return s.watcher.Start()
	}
	return nil
}

func (s *Store) Close() error {
	if s.watcher != nil {
		return s.watcher.Close()
	}
	return nil
}

func (s *Store) Pool() *x509.CertPool {
	s.access.RLock()
	defer s.access.RUnlock()
	return s.currentPool
}

func (s *Store) update() error {
	s.access.Lock()
	defer s.access.Unlock()
	var currentPool *x509.CertPool
	if s.systemPool == nil {
		currentPool = x509.NewCertPool()
	} else {
		currentPool = s.systemPool.Clone()
	}
	if s.certificate != "" {
		if !currentPool.AppendCertsFromPEM([]byte(s.certificate)) {
			return E.New("invalid certificate PEM strings")
		}
	}
	for _, path := range s.certificatePaths {
		pemContent, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !currentPool.AppendCertsFromPEM(pemContent) {
			return E.New("invalid certificate PEM file: ", path)
		}
	}
	var firstErr error
	for _, directoryPath := range s.certificateDirectoryPaths {
		directoryEntries, err := readUniqueDirectoryEntries(directoryPath)
		if err != nil {
			if firstErr == nil && !os.IsNotExist(err) {
				firstErr = E.Cause(err, "invalid certificate directory: ", directoryPath)
			}
			continue
		}
		for _, directoryEntry := range directoryEntries {
			pemContent, err := os.ReadFile(filepath.Join(directoryPath, directoryEntry.Name()))
			if err == nil {
				currentPool.AppendCertsFromPEM(pemContent)
			}
		}
	}
	if firstErr != nil {
		return firstErr
	}
	s.currentPool = currentPool
	return nil
}

func readUniqueDirectoryEntries(dir string) ([]fs.DirEntry, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	uniq := files[:0]
	for _, f := range files {
		if !isSameDirSymlink(f, dir) {
			uniq = append(uniq, f)
		}
	}
	return uniq, nil
}

func isSameDirSymlink(f fs.DirEntry, dir string) bool {
	if f.Type()&fs.ModeSymlink == 0 {
		return false
	}
	target, err := os.Readlink(filepath.Join(dir, f.Name()))
	return err == nil && !strings.Contains(target, "/")
}
