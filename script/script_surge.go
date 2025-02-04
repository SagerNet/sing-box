package script

import (
	"context"
	"net/http"
	"sync"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing-box/script/modules/surge"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"

	"github.com/adhocore/gronx"
	"github.com/dop251/goja"
)

const defaultSurgeScriptTimeout = 10 * time.Second

var _ adapter.SurgeScript = (*SurgeScript)(nil)

type SurgeScript struct {
	ctx    context.Context
	logger logger.ContextLogger
	tag    string
	source Source

	cronExpression string
	cronTimeout    time.Duration
	cronArguments  []string
	cronTimer      *time.Timer
	cronDone       chan struct{}
}

func NewSurgeScript(ctx context.Context, logger logger.ContextLogger, options option.Script) (adapter.Script, error) {
	source, err := NewSource(ctx, logger, options)
	if err != nil {
		return nil, err
	}
	cronOptions := common.PtrValueOrDefault(options.SurgeOptions.CronOptions)
	if cronOptions.Expression != "" {
		if !gronx.IsValid(cronOptions.Expression) {
			return nil, E.New("invalid cron expression: ", cronOptions.Expression)
		}
	}
	return &SurgeScript{
		ctx:            ctx,
		logger:         logger,
		tag:            options.Tag,
		source:         source,
		cronExpression: cronOptions.Expression,
		cronTimeout:    time.Duration(cronOptions.Timeout),
		cronArguments:  cronOptions.Arguments,
		cronDone:       make(chan struct{}),
	}, nil
}

func (s *SurgeScript) Type() string {
	return C.ScriptTypeSurge
}

func (s *SurgeScript) Tag() string {
	return s.tag
}

func (s *SurgeScript) StartContext(ctx context.Context, startContext *adapter.HTTPStartContext) error {
	return s.source.StartContext(ctx, startContext)
}

func (s *SurgeScript) PostStart() error {
	err := s.source.PostStart()
	if err != nil {
		return err
	}
	if s.cronExpression != "" {
		go s.loopCronEvents()
	}
	return nil
}

func (s *SurgeScript) loopCronEvents() {
	s.logger.Debug("starting event")
	err := s.ExecuteGeneric(s.ctx, "cron", s.cronTimeout, s.cronArguments)
	if err != nil {
		s.logger.Error(E.Cause(err, "running event"))
	}
	nextTick, err := gronx.NextTick(s.cronExpression, false)
	if err != nil {
		s.logger.Error(E.Cause(err, "determine next tick"))
		return
	}
	s.cronTimer = time.NewTimer(nextTick.Sub(time.Now()))
	s.logger.Debug("next event at: ", nextTick.Format(log.DefaultTimeFormat))
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.cronDone:
			return
		case <-s.cronTimer.C:
			s.logger.Debug("starting event")
			err = s.ExecuteGeneric(s.ctx, "cron", s.cronTimeout, s.cronArguments)
			if err != nil {
				s.logger.Error(E.Cause(err, "running event"))
			}
			nextTick, err = gronx.NextTick(s.cronExpression, false)
			if err != nil {
				s.logger.Error(E.Cause(err, "determine next tick"))
				return
			}
			s.cronTimer.Reset(nextTick.Sub(time.Now()))
			s.logger.Debug("configured next event at: ", nextTick)
		}
	}
}

func (s *SurgeScript) Close() error {
	err := s.source.Close()
	if s.cronTimer != nil {
		s.cronTimer.Stop()
		close(s.cronDone)
	}
	return err
}

func (s *SurgeScript) ExecuteGeneric(ctx context.Context, scriptType string, timeout time.Duration, arguments []string) error {
	program := s.source.Program()
	if program == nil {
		return E.New("invalid script")
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	runtime := NewRuntime(ctx, cancel)
	SetModules(runtime, ctx, s.logger, cancel, s.tag)
	surge.Enable(runtime, scriptType, arguments)
	if timeout == 0 {
		timeout = defaultSurgeScriptTimeout
	}
	ctx, timeoutCancel := context.WithTimeout(ctx, timeout)
	defer timeoutCancel()
	done := make(chan struct{})
	doneFunc := common.OnceFunc(func() {
		close(done)
	})
	runtime.Set("done", func(call goja.FunctionCall) goja.Value {
		doneFunc()
		return goja.Undefined()
	})
	var (
		access    sync.Mutex
		scriptErr error
	)
	go func() {
		_, err := runtime.RunProgram(program)
		if err != nil {
			access.Lock()
			scriptErr = err
			access.Unlock()
			doneFunc()
		}
	}()
	select {
	case <-ctx.Done():
		runtime.Interrupt(ctx.Err())
		return ctx.Err()
	case <-done:
		access.Lock()
		defer access.Unlock()
		if scriptErr != nil {
			runtime.Interrupt(scriptErr)
		} else {
			runtime.Interrupt("script done")
		}
	}
	return scriptErr
}

func (s *SurgeScript) ExecuteHTTPRequest(ctx context.Context, timeout time.Duration, request *http.Request, body []byte, binaryBody bool, arguments []string) (*adapter.HTTPRequestScriptResult, error) {
	program := s.source.Program()
	if program == nil {
		return nil, E.New("invalid script")
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	runtime := NewRuntime(ctx, cancel)
	SetModules(runtime, ctx, s.logger, cancel, s.tag)
	surge.Enable(runtime, "http-request", arguments)
	if timeout == 0 {
		timeout = defaultSurgeScriptTimeout
	}
	ctx, timeoutCancel := context.WithTimeout(ctx, timeout)
	defer timeoutCancel()
	runtime.ClearInterrupt()
	requestObject := runtime.NewObject()
	requestObject.Set("url", request.URL.String())
	requestObject.Set("method", request.Method)
	requestObject.Set("headers", jsc.HeadersToValue(runtime, request.Header))
	if !binaryBody {
		requestObject.Set("body", string(body))
	} else {
		requestObject.Set("body", jsc.NewUint8Array(runtime, body))
	}
	requestObject.Set("id", F.ToString(uintptr(unsafe.Pointer(request))))
	runtime.Set("request", requestObject)
	done := make(chan struct{})
	doneFunc := common.OnceFunc(func() {
		close(done)
	})
	var (
		access    sync.Mutex
		result    adapter.HTTPRequestScriptResult
		scriptErr error
	)
	runtime.Set("done", func(call goja.FunctionCall) goja.Value {
		defer doneFunc()
		resultObject := jsc.AssertObject(runtime, call.Argument(0), "done() argument", true)
		if resultObject == nil {
			panic(runtime.NewGoError(E.New("request rejected by script")))
		}
		access.Lock()
		defer access.Unlock()
		result.URL = jsc.AssertString(runtime, resultObject.Get("url"), "url", true)
		result.Headers = jsc.AssertHTTPHeader(runtime, resultObject.Get("headers"), "headers")
		result.Body = jsc.AssertStringBinary(runtime, resultObject.Get("body"), "body", true)
		responseObject := jsc.AssertObject(runtime, resultObject.Get("response"), "response", true)
		if responseObject != nil {
			result.Response = &adapter.HTTPRequestScriptResponse{
				Status:  int(jsc.AssertInt(runtime, responseObject.Get("status"), "status", true)),
				Headers: jsc.AssertHTTPHeader(runtime, responseObject.Get("headers"), "headers"),
				Body:    jsc.AssertStringBinary(runtime, responseObject.Get("body"), "body", true),
			}
		}
		return goja.Undefined()
	})
	go func() {
		_, err := runtime.RunProgram(program)
		if err != nil {
			access.Lock()
			scriptErr = err
			access.Unlock()
			doneFunc()
		}
	}()
	select {
	case <-ctx.Done():
		runtime.Interrupt(ctx.Err())
		return nil, ctx.Err()
	case <-done:
		access.Lock()
		defer access.Unlock()
		if scriptErr != nil {
			runtime.Interrupt(scriptErr)
		} else {
			runtime.Interrupt("script done")
		}
	}
	return &result, scriptErr
}

func (s *SurgeScript) ExecuteHTTPResponse(ctx context.Context, timeout time.Duration, request *http.Request, response *http.Response, body []byte, binaryBody bool, arguments []string) (*adapter.HTTPResponseScriptResult, error) {
	program := s.source.Program()
	if program == nil {
		return nil, E.New("invalid script")
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	runtime := NewRuntime(ctx, cancel)
	SetModules(runtime, ctx, s.logger, cancel, s.tag)
	surge.Enable(runtime, "http-response", arguments)
	if timeout == 0 {
		timeout = defaultSurgeScriptTimeout
	}
	ctx, timeoutCancel := context.WithTimeout(ctx, timeout)
	defer timeoutCancel()
	runtime.ClearInterrupt()
	requestObject := runtime.NewObject()
	requestObject.Set("url", request.URL.String())
	requestObject.Set("method", request.Method)
	requestObject.Set("headers", jsc.HeadersToValue(runtime, request.Header))
	requestObject.Set("id", F.ToString(uintptr(unsafe.Pointer(request))))
	runtime.Set("request", requestObject)

	responseObject := runtime.NewObject()
	responseObject.Set("status", response.StatusCode)
	responseObject.Set("headers", jsc.HeadersToValue(runtime, response.Header))
	if !binaryBody {
		responseObject.Set("body", string(body))
	} else {
		responseObject.Set("body", jsc.NewUint8Array(runtime, body))
	}
	runtime.Set("response", responseObject)

	done := make(chan struct{})
	doneFunc := common.OnceFunc(func() {
		close(done)
	})
	var (
		access    sync.Mutex
		result    adapter.HTTPResponseScriptResult
		scriptErr error
	)
	runtime.Set("done", func(call goja.FunctionCall) goja.Value {
		resultObject := jsc.AssertObject(runtime, call.Argument(0), "done() argument", true)
		if resultObject == nil {
			panic(runtime.NewGoError(E.New("response rejected by script")))
		}
		access.Lock()
		defer access.Unlock()
		result.Status = int(jsc.AssertInt(runtime, resultObject.Get("status"), "status", true))
		result.Headers = jsc.AssertHTTPHeader(runtime, resultObject.Get("headers"), "headers")
		result.Body = jsc.AssertStringBinary(runtime, resultObject.Get("body"), "body", true)
		doneFunc()
		return goja.Undefined()
	})
	go func() {
		_, err := runtime.RunProgram(program)
		if err != nil {
			access.Lock()
			scriptErr = err
			access.Unlock()
			doneFunc()
		}
	}()
	select {
	case <-ctx.Done():
		runtime.Interrupt(ctx.Err())
		return nil, ctx.Err()
	case <-done:
		access.Lock()
		defer access.Unlock()
		if scriptErr != nil {
			runtime.Interrupt(scriptErr)
		} else {
			runtime.Interrupt("script done")
		}
		return &result, scriptErr
	}
}
