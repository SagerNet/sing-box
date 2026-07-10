package netns

import (
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

var _ adapter.NetworkNamespaceManager = (*Manager)(nil)

type Manager struct {
	logger     logger.ContextLogger
	namespaces []option.NetworkNamespace
	holderArgs []string
	paths      map[string]string
	holders    []*holder
}

func NewManager(logger logger.ContextLogger, namespaces []option.NetworkNamespace, holderArgs []string) (*Manager, error) {
	paths := make(map[string]string)
	for _, namespace := range namespaces {
		if namespace.Tag == "" {
			return nil, E.New("network namespace: missing tag")
		}
		_, duplicated := paths[namespace.Tag]
		if duplicated {
			return nil, E.New("network namespace: duplicated tag: ", namespace.Tag)
		}
		switch namespace.Type {
		case C.NetNsTypeDefault:
			if namespace.DefaultOptions.Path == "" {
				return nil, E.New("network namespace[", namespace.Tag, "]: missing path")
			}
			paths[namespace.Tag] = namespace.DefaultOptions.Path
		case C.NetNsTypeUnshare:
			paths[namespace.Tag] = ""
		}
	}
	return &Manager{
		logger:     logger,
		namespaces: namespaces,
		holderArgs: holderArgs,
		paths:      paths,
	}, nil
}

func (m *Manager) Name() string {
	return "netns"
}

func (m *Manager) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateInitialize {
		return nil
	}
	return m.start()
}

func (m *Manager) Close() error {
	return m.close()
}

func (m *Manager) ResolvePath(nameOrPath string) string {
	path, loaded := m.paths[nameOrPath]
	if loaded && path != "" {
		return path
	}
	return nameOrPath
}

func Hold() {
	buffer := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(buffer)
		if err != nil {
			os.Exit(0)
		}
	}
}
