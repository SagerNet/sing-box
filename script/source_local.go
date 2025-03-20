//go:build with_script

package script

import (
	"context"
	"os"
	"path/filepath"

	"github.com/sagernet/fswatch"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service/filemanager"

	"github.com/dop251/goja"
)

var _ Source = (*LocalSource)(nil)

type LocalSource struct {
	ctx     context.Context
	logger  logger.Logger
	tag     string
	program *goja.Program
	watcher *fswatch.Watcher
}

func NewLocalSource(ctx context.Context, logger logger.Logger, options option.Script) (*LocalSource, error) {
	script := &LocalSource{
		ctx:    ctx,
		logger: logger,
		tag:    options.Tag,
	}
	filePath := filemanager.BasePath(ctx, options.LocalOptions.Path)
	filePath, _ = filepath.Abs(options.LocalOptions.Path)
	err := script.reloadFile(filePath)
	if err != nil {
		return nil, err
	}
	watcher, err := fswatch.NewWatcher(fswatch.Options{
		Path: []string{filePath},
		Callback: func(path string) {
			uErr := script.reloadFile(path)
			if uErr != nil {
				logger.Error(E.Cause(uErr, "reload script ", path))
			}
		},
	})
	if err != nil {
		return nil, err
	}
	script.watcher = watcher
	return script, nil
}

func (s *LocalSource) StartContext(ctx context.Context, startContext *adapter.HTTPStartContext) error {
	if s.watcher != nil {
		err := s.watcher.Start()
		if err != nil {
			s.logger.Error(E.Cause(err, "watch script file"))
		}
	}
	return nil
}

func (s *LocalSource) reloadFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	program, err := goja.Compile("script:"+s.tag, string(content), false)
	if err != nil {
		return E.Cause(err, "compile ", path)
	}
	if s.program != nil {
		s.logger.Info("reloaded from ", path)
	}
	s.program = program
	return nil
}

func (s *LocalSource) PostStart() error {
	return nil
}

func (s *LocalSource) Program() *goja.Program {
	return s.program
}

func (s *LocalSource) Close() error {
	return s.watcher.Close()
}
