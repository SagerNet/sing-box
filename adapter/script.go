package adapter

import (
	"context"
	"net/http"
)

type ScriptManager interface {
	Lifecycle
	Scripts() []Script
	// Script(name string) (Script, bool)
}

type Script interface {
	Type() string
	Tag() string
	StartContext(ctx context.Context, startContext *HTTPStartContext) error
	PostStart() error
	Close() error
}

type GenericScript interface {
	Script
	Run(ctx context.Context) error
}

type HTTPScript interface {
	Script
	Match(requestURL string) bool
	RequiresBody() bool
	MaxSize() int64
}

type HTTPRequestScript interface {
	HTTPScript
	Run(ctx context.Context, request *http.Request, body []byte) (*HTTPRequestScriptResult, error)
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

type HTTPResponseScript interface {
	HTTPScript
	Run(ctx context.Context, request *http.Request, response *http.Response, body []byte) (*HTTPResponseScriptResult, error)
}

type HTTPResponseScriptResult struct {
	Status  int
	Headers http.Header
	Body    []byte
}
