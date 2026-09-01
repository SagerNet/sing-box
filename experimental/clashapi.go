package experimental

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

type ClashServerConstructor = func(ctx context.Context, logFactory log.ObservableFactory, options option.ClashAPIOptions) (adapter.LifecycleService, error)

var clashServerConstructor ClashServerConstructor

func RegisterClashServerConstructor(constructor ClashServerConstructor) {
	clashServerConstructor = constructor
}

func NewClashServer(ctx context.Context, logFactory log.ObservableFactory, options option.ClashAPIOptions) (adapter.LifecycleService, error) {
	if clashServerConstructor == nil {
		return nil, os.ErrInvalid
	}
	return clashServerConstructor(ctx, logFactory, options)
}
