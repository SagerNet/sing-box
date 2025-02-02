package script

import (
	"context"
	"net/http"
	"regexp"
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

var _ adapter.HTTPRequestScript = (*SurgeHTTPRequestScript)(nil)

type SurgeHTTPRequestScript struct {
	GenericScript
	pattern        *regexp.Regexp
	requiresBody   bool
	maxSize        int64
	binaryBodyMode bool
}

func NewSurgeHTTPRequestScript(ctx context.Context, logger logger.ContextLogger, options option.Script) (*SurgeHTTPRequestScript, error) {
	source, err := NewSource(ctx, logger, options)
	if err != nil {
		return nil, err
	}
	pattern, err := regexp.Compile(options.HTTPOptions.Pattern)
	if err != nil {
		return nil, E.Cause(err, "parse pattern")
	}
	return &SurgeHTTPRequestScript{
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

func (s *SurgeHTTPRequestScript) Type() string {
	return C.ScriptTypeSurgeHTTPRequest
}

func (s *SurgeHTTPRequestScript) Tag() string {
	return s.tag
}

func (s *SurgeHTTPRequestScript) StartContext(ctx context.Context, startContext *adapter.HTTPStartContext) error {
	return s.source.StartContext(ctx, startContext)
}

func (s *SurgeHTTPRequestScript) PostStart() error {
	return s.source.PostStart()
}

func (s *SurgeHTTPRequestScript) Close() error {
	return s.source.Close()
}

func (s *SurgeHTTPRequestScript) Match(requestURL string) bool {
	return s.pattern.MatchString(requestURL)
}

func (s *SurgeHTTPRequestScript) RequiresBody() bool {
	return s.requiresBody
}

func (s *SurgeHTTPRequestScript) MaxSize() int64 {
	return s.maxSize
}

func (s *SurgeHTTPRequestScript) Run(ctx context.Context, request *http.Request, body []byte) (*adapter.HTTPRequestScriptResult, error) {
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
	return ExecuteSurgeHTTPRequest(vm, program, ctx, s.timeout, request, body, s.binaryBodyMode)
}

func ExecuteSurgeHTTPRequest(vm *goja.Runtime, program *goja.Program, ctx context.Context, timeout time.Duration, request *http.Request, body []byte, binaryBody bool) (*adapter.HTTPRequestScriptResult, error) {
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
	if !binaryBody {
		requestObject.Set("body", string(body))
	} else {
		requestObject.Set("body", jsc.NewUint8Array(vm, body))
	}
	requestObject.Set("id", F.ToString(uintptr(unsafe.Pointer(request))))
	vm.Set("request", requestObject)
	done := make(chan struct{})
	doneFunc := common.OnceFunc(func() {
		close(done)
	})
	var result adapter.HTTPRequestScriptResult
	vm.Set("done", func(call goja.FunctionCall) goja.Value {
		defer doneFunc()
		resultObject := jsc.AssertObject(vm, call.Argument(0), "done() argument", true)
		if resultObject == nil {
			panic(vm.NewGoError(E.New("request rejected by script")))
		}
		result.URL = jsc.AssertString(vm, resultObject.Get("url"), "url", true)
		result.Headers = jsc.AssertHTTPHeader(vm, resultObject.Get("headers"), "headers")
		result.Body = jsc.AssertStringBinary(vm, resultObject.Get("body"), "body", true)
		responseObject := jsc.AssertObject(vm, resultObject.Get("response"), "response", true)
		if responseObject != nil {
			result.Response = &adapter.HTTPRequestScriptResponse{
				Status:  int(jsc.AssertInt(vm, responseObject.Get("status"), "status", true)),
				Headers: jsc.AssertHTTPHeader(vm, responseObject.Get("headers"), "headers"),
				Body:    jsc.AssertStringBinary(vm, responseObject.Get("body"), "body", true),
			}
		}
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
		return nil, ctx.Err()
	case <-done:
		if err != nil {
			vm.Interrupt(err)
		} else {
			vm.Interrupt("script done")
		}
	}
	return &result, err
}
