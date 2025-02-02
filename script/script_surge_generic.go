package script

import (
	"context"
	"runtime"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/locale"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing-box/script/modules/console"
	"github.com/sagernet/sing-box/script/modules/eventloop"
	"github.com/sagernet/sing-box/script/modules/require"
	"github.com/sagernet/sing-box/script/modules/sghttp"
	"github.com/sagernet/sing-box/script/modules/sgnotification"
	"github.com/sagernet/sing-box/script/modules/sgstore"
	"github.com/sagernet/sing-box/script/modules/sgutils"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/ntp"

	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
)

const defaultScriptTimeout = 10 * time.Second

var _ adapter.GenericScript = (*GenericScript)(nil)

type GenericScript struct {
	logger    logger.ContextLogger
	tag       string
	timeout   time.Duration
	arguments []any
	source    Source
}

func NewSurgeGenericScript(ctx context.Context, logger logger.ContextLogger, options option.Script) (*GenericScript, error) {
	source, err := NewSource(ctx, logger, options)
	if err != nil {
		return nil, err
	}
	return &GenericScript{
		logger:    logger,
		tag:       options.Tag,
		timeout:   time.Duration(options.Timeout),
		arguments: options.Arguments,
		source:    source,
	}, nil
}

func (s *GenericScript) Type() string {
	return C.ScriptTypeSurgeGeneric
}

func (s *GenericScript) Tag() string {
	return s.tag
}

func (s *GenericScript) StartContext(ctx context.Context, startContext *adapter.HTTPStartContext) error {
	return s.source.StartContext(ctx, startContext)
}

func (s *GenericScript) PostStart() error {
	return s.source.PostStart()
}

func (s *GenericScript) Close() error {
	return s.source.Close()
}

func (s *GenericScript) Run(ctx context.Context) error {
	program := s.source.Program()
	if program == nil {
		return E.New("invalid script")
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	vm := NewRuntime(ctx, s.logger, cancel)
	err := SetSurgeModules(vm, ctx, s.logger, cancel, s.Tag(), s.Type(), s.arguments)
	if err != nil {
		return err
	}
	return ExecuteSurgeGeneral(vm, program, ctx, s.timeout)
}

func NewRuntime(ctx context.Context, logger logger.ContextLogger, cancel context.CancelCauseFunc) *goja.Runtime {
	vm := goja.New()
	if timeFunc := ntp.TimeFuncFromContext(ctx); timeFunc != nil {
		vm.SetTimeSource(timeFunc)
	}
	vm.SetParserOptions(parser.WithDisableSourceMaps)
	registry := require.NewRegistry(require.WithLoader(func(path string) ([]byte, error) {
		return nil, E.New("unsupported usage")
	}))
	registry.Enable(vm)
	registry.RegisterNodeModule(console.ModuleName, console.Require(ctx, logger))
	console.Enable(vm)
	eventloop.Enable(vm, cancel)
	return vm
}

func SetSurgeModules(vm *goja.Runtime, ctx context.Context, logger logger.Logger, errorHandler func(error), tag string, scriptType string, arguments []any) error {
	script := vm.NewObject()
	script.Set("name", F.ToString("script:", tag))
	script.Set("startTime", jsc.TimeToValue(vm, time.Now()))
	script.Set("type", scriptType)
	vm.Set("$script", script)

	environment := vm.NewObject()
	var system string
	switch runtime.GOOS {
	case "ios":
		system = "iOS"
	case "darwin":
		system = "macOS"
	case "tvos":
		system = "tvOS"
	case "linux":
		system = "Linux"
	case "android":
		system = "Android"
	case "windows":
		system = "Windows"
	default:
		system = runtime.GOOS
	}
	environment.Set("system", system)
	environment.Set("surge-build", "N/A")
	environment.Set("surge-version", "sing-box "+C.Version)
	environment.Set("language", locale.Current().Locale)
	environment.Set("device-model", "N/A")
	vm.Set("$environment", environment)

	sgstore.Enable(vm, ctx)
	sgutils.Enable(vm)
	sghttp.Enable(vm, ctx, errorHandler)
	sgnotification.Enable(vm, ctx, logger)

	vm.Set("$argument", arguments)
	return nil
}

func ExecuteSurgeGeneral(vm *goja.Runtime, program *goja.Program, ctx context.Context, timeout time.Duration) error {
	if timeout == 0 {
		timeout = defaultScriptTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	vm.ClearInterrupt()
	done := make(chan struct{})
	doneFunc := common.OnceFunc(func() {
		close(done)
	})
	vm.Set("done", func(call goja.FunctionCall) goja.Value {
		doneFunc()
		return goja.Undefined()
	})
	var err error
	go func() {
		_, err = vm.RunProgram(program)
		if err != nil {
			doneFunc()
		}
	}()
	select {
	case <-ctx.Done():
		vm.Interrupt(ctx.Err())
		return ctx.Err()
	case <-done:
		if err != nil {
			vm.Interrupt(err)
		} else {
			vm.Interrupt("script done")
		}
	}
	return err
}
