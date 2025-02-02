package script

import (
	"context"
	"net/http"
	"regexp"
	"sync"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"

	"github.com/dop251/goja"
)

var _ adapter.HTTPResponseScript = (*SurgeHTTPResponseScript)(nil)

type SurgeHTTPResponseScript struct {
	GenericScript
	pattern        *regexp.Regexp
	requiresBody   bool
	maxSize        int64
	binaryBodyMode bool
}

func NewSurgeHTTPResponseScript(ctx context.Context, logger logger.ContextLogger, options option.Script) (*SurgeHTTPResponseScript, error) {
	source, err := NewSource(ctx, logger, options)
	if err != nil {
		return nil, err
	}
	pattern, err := regexp.Compile(options.HTTPOptions.Pattern)
	if err != nil {
		return nil, E.Cause(err, "parse pattern")
	}
	return &SurgeHTTPResponseScript{
		GenericScript: GenericScript{
			logger:    logger,
			tag:       options.Tag,
			timeout:   time.Duration(options.Timeout),
			arguments: options.Arguments,
			source:    source,
		},
		pattern:        pattern,
		requiresBody:   options.HTTPOptions.RequiresBody,
		maxSize:        options.HTTPOptions.MaxSize,
		binaryBodyMode: options.HTTPOptions.BinaryBodyMode,
	}, nil
}

func (s *SurgeHTTPResponseScript) Type() string {
	return C.ScriptTypeSurgeHTTPResponse
}

func (s *SurgeHTTPResponseScript) Tag() string {
	return s.tag
}

func (s *SurgeHTTPResponseScript) StartContext(ctx context.Context, startContext *adapter.HTTPStartContext) error {
	return s.source.StartContext(ctx, startContext)
}

func (s *SurgeHTTPResponseScript) PostStart() error {
	return s.source.PostStart()
}

func (s *SurgeHTTPResponseScript) Close() error {
	return s.source.Close()
}

func (s *SurgeHTTPResponseScript) Match(requestURL string) bool {
	return s.pattern.MatchString(requestURL)
}

func (s *SurgeHTTPResponseScript) RequiresBody() bool {
	return s.requiresBody
}

func (s *SurgeHTTPResponseScript) MaxSize() int64 {
	return s.maxSize
}

func (s *SurgeHTTPResponseScript) Run(ctx context.Context, request *http.Request, response *http.Response, body []byte) (*adapter.HTTPResponseScriptResult, error) {
	program := s.source.Program()
	if program == nil {
		return nil, E.New("invalid script")
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	vm := NewRuntime(ctx, s.logger, cancel)
	err := SetSurgeModules(vm, ctx, s.logger, cancel, s.Tag(), s.Type(), s.arguments)
	if err != nil {
		return nil, err
	}
	return ExecuteSurgeHTTPResponse(vm, program, ctx, s.timeout, request, response, body, s.binaryBodyMode)
}

func ExecuteSurgeHTTPResponse(vm *goja.Runtime, program *goja.Program, ctx context.Context, timeout time.Duration, request *http.Request, response *http.Response, body []byte, binaryBody bool) (*adapter.HTTPResponseScriptResult, error) {
	if timeout == 0 {
		timeout = defaultScriptTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	vm.ClearInterrupt()
	requestObject := vm.NewObject()
	requestObject.Set("url", request.URL.String())
	requestObject.Set("method", request.Method)
	requestObject.Set("headers", jsc.HeadersToValue(vm, request.Header))
	requestObject.Set("id", F.ToString(uintptr(unsafe.Pointer(request))))
	vm.Set("request", requestObject)

	responseObject := vm.NewObject()
	responseObject.Set("status", response.StatusCode)
	responseObject.Set("headers", jsc.HeadersToValue(vm, response.Header))
	if !binaryBody {
		responseObject.Set("body", string(body))
	} else {
		responseObject.Set("body", jsc.NewUint8Array(vm, body))
	}
	vm.Set("response", responseObject)

	done := make(chan struct{})
	doneFunc := common.OnceFunc(func() {
		close(done)
	})
	var (
		access sync.Mutex
		result adapter.HTTPResponseScriptResult
	)
	vm.Set("done", func(call goja.FunctionCall) goja.Value {
		resultObject := jsc.AssertObject(vm, call.Argument(0), "done() argument", true)
		if resultObject == nil {
			panic(vm.NewGoError(E.New("response rejected by script")))
		}
		access.Lock()
		defer access.Unlock()
		result.Status = int(jsc.AssertInt(vm, resultObject.Get("status"), "status", true))
		result.Headers = jsc.AssertHTTPHeader(vm, resultObject.Get("headers"), "headers")
		result.Body = jsc.AssertStringBinary(vm, resultObject.Get("body"), "body", true)
		doneFunc()
		return goja.Undefined()
	})
	var scriptErr error
	go func() {
		_, err := vm.RunProgram(program)
		if err != nil {
			access.Lock()
			scriptErr = err
			access.Unlock()
			doneFunc()
		}
	}()
	select {
	case <-ctx.Done():
		println(1)
		vm.Interrupt(ctx.Err())
		return nil, ctx.Err()
	case <-done:
		access.Lock()
		defer access.Unlock()
		if scriptErr != nil {
			vm.Interrupt(scriptErr)
		} else {
			vm.Interrupt("script done")
		}
		return &result, scriptErr
	}
}
