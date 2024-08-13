package provider

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/service/filemanager"
)

func New(ctx context.Context, router adapter.Router, logger log.ContextLogger, options option.OutboundProvider) (adapter.OutboundProvider, error) {
	if options.Path == "" {
		return nil, E.New("provider path missing")
	}
	path, _ := C.FindPath(options.Path)
	if foundPath, loaded := C.FindPath(path); loaded {
		path = foundPath
	}
	if !rw.FileExists(path) {
		path = filemanager.BasePath(ctx, path)
	}
	if stat, err := os.Stat(path); err == nil {
		if stat.IsDir() {
			return nil, E.New("provider path is a directory: ", path)
		}
		if stat.Size() == 0 {
			os.Remove(path)
		}
	}
	switch options.Type {
	case C.ProviderTypeLocal:
		return NewLocalProvider(ctx, router, logger, options, path)
	case C.ProviderTypeRemote:
		return NewRemoteProvider(ctx, router, logger, options, path)
	default:
		return nil, E.New("invalid provider type")
	}
}
