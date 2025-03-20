//go:build with_script

package script

import (
	"context"

	"github.com/sagernet/sing-box/script/modules/boxctx"
	"github.com/sagernet/sing-box/script/modules/console"
	"github.com/sagernet/sing-box/script/modules/eventloop"
	"github.com/sagernet/sing-box/script/modules/require"
	"github.com/sagernet/sing-box/script/modules/surge"
	"github.com/sagernet/sing-box/script/modules/url"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/ntp"

	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
)

func NewRuntime(ctx context.Context, cancel context.CancelCauseFunc) *goja.Runtime {
	vm := goja.New()
	if timeFunc := ntp.TimeFuncFromContext(ctx); timeFunc != nil {
		vm.SetTimeSource(timeFunc)
	}
	vm.SetParserOptions(parser.WithDisableSourceMaps)
	registry := require.NewRegistry(require.WithLoader(func(path string) ([]byte, error) {
		return nil, E.New("unsupported usage")
	}))
	registry.Enable(vm)
	registry.RegisterNodeModule(console.ModuleName, console.Require)
	registry.RegisterNodeModule(url.ModuleName, url.Require)
	registry.RegisterNativeModule(boxctx.ModuleName, boxctx.Require)
	registry.RegisterNativeModule(surge.ModuleName, surge.Require)
	console.Enable(vm)
	url.Enable(vm)
	eventloop.Enable(vm, cancel)
	return vm
}

func SetModules(runtime *goja.Runtime, ctx context.Context, logger logger.ContextLogger, errorHandler func(error), tag string) {
	boxctx.Enable(runtime, &boxctx.Context{
		Context:      ctx,
		Logger:       logger,
		Tag:          tag,
		ErrorHandler: errorHandler,
	})
}
