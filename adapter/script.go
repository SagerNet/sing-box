package adapter

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type ScriptManager interface {
	Lifecycle
	Scripts() []Script
	Script(name string) (Script, bool)
	SurgeCache() *SurgeInMemoryCache
}

type SurgeInMemoryCache struct {
	sync.RWMutex
	Data map[string]string
}

type Script interface {
	Type() string
	Tag() string
	StartContext(ctx context.Context, startContext *HTTPStartContext) error
	PostStart() error
	Close() error
}

type SurgeScript interface {
	Script
	ExecuteGeneric(ctx context.Context, scriptType string, timeout time.Duration, arguments []string) error
	ExecuteHTTPRequest(ctx context.Context, timeout time.Duration, request *http.Request, body []byte, binaryBody bool, arguments []string) (*HTTPRequestScriptResult, error)
	ExecuteHTTPResponse(ctx context.Context, timeout time.Duration, request *http.Request, response *http.Response, body []byte, binaryBody bool, arguments []string) (*HTTPResponseScriptResult, error)
}

type HTTPRequestScriptResult struct {
	URL      string
	Headers  http.Header
	Body     []byte
	Response *HTTPRequestScriptResponse
}

type HTTPRequestScriptResponse struct {
	Status  int
	Headers http.Header
	Body    []byte
}

type HTTPResponseScriptResult struct {
	Status  int
	Headers http.Header
	Body    []byte
}
