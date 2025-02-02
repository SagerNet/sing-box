package sghttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"

	"github.com/sagernet/sing-box/script/jsc"
	F "github.com/sagernet/sing/common/format"

	"github.com/dop251/goja"
	"golang.org/x/net/publicsuffix"
)

type SurgeHTTP struct {
	vm           *goja.Runtime
	ctx          context.Context
	cookieAccess sync.RWMutex
	cookieJar    *cookiejar.Jar
	errorHandler func(error)
}

func Enable(vm *goja.Runtime, ctx context.Context, errorHandler func(error)) {
	sgHTTP := &SurgeHTTP{
		vm:           vm,
		ctx:          ctx,
		errorHandler: errorHandler,
	}
	httpObject := vm.NewObject()
	httpObject.Set("get", sgHTTP.request(http.MethodGet))
	httpObject.Set("post", sgHTTP.request(http.MethodPost))
	httpObject.Set("put", sgHTTP.request(http.MethodPut))
	httpObject.Set("delete", sgHTTP.request(http.MethodDelete))
	httpObject.Set("head", sgHTTP.request(http.MethodHead))
	httpObject.Set("options", sgHTTP.request(http.MethodOptions))
	httpObject.Set("patch", sgHTTP.request(http.MethodPatch))
	httpObject.Set("trace", sgHTTP.request(http.MethodTrace))
	vm.Set("$http", httpObject)
}

func (s *SurgeHTTP) request(method string) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) != 2 {
			panic(s.vm.NewTypeError("invalid arguments"))
		}
		var (
			url          string
			headers      http.Header
			body         []byte
			timeout      = 5 * time.Second
			insecure     bool
			autoCookie   bool
			autoRedirect bool
			// policy       string
			binaryMode bool
		)
		switch optionsValue := call.Argument(0).(type) {
		case goja.String:
			url = optionsValue.String()
		case *goja.Object:
			url = jsc.AssertString(s.vm, optionsValue.Get("url"), "options.url", false)
			headers = jsc.AssertHTTPHeader(s.vm, optionsValue.Get("headers"), "option.headers")
			body = jsc.AssertStringBinary(s.vm, optionsValue.Get("body"), "options.body", true)
			timeoutInt := jsc.AssertInt(s.vm, optionsValue.Get("timeout"), "options.timeout", true)
			if timeoutInt > 0 {
				timeout = time.Duration(timeoutInt) * time.Second
			}
			insecure = jsc.AssertBool(s.vm, optionsValue.Get("insecure"), "options.insecure", true)
			autoCookie = jsc.AssertBool(s.vm, optionsValue.Get("auto-cookie"), "options.auto-cookie", true)
			autoRedirect = jsc.AssertBool(s.vm, optionsValue.Get("auto-redirect"), "options.auto-redirect", true)
			// policy = jsc.AssertString(s.vm, optionsValue.Get("policy"), "options.policy", true)
			binaryMode = jsc.AssertBool(s.vm, optionsValue.Get("binary-mode"), "options.binary-mode", true)
		default:
			panic(s.vm.NewTypeError(F.ToString("invalid argument: options: expected string or object, but got ", optionsValue)))
		}
		callback := jsc.AssertFunction(s.vm, call.Argument(1), "callback")
		httpClient := &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: insecure,
				},
				ForceAttemptHTTP2: true,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if autoRedirect {
					return nil
				}
				return http.ErrUseLastResponse
			},
		}
		if autoCookie {
			s.cookieAccess.Lock()
			if s.cookieJar == nil {
				s.cookieJar, _ = cookiejar.New(&cookiejar.Options{
					PublicSuffixList: publicsuffix.List,
				})
			}
			httpClient.Jar = s.cookieJar
			s.cookieAccess.Lock()
		}
		request, err := http.NewRequestWithContext(s.ctx, method, url, bytes.NewReader(body))
		if host := headers.Get("Host"); host != "" {
			request.Host = host
			headers.Del("Host")
		}
		request.Header = headers
		if err != nil {
			panic(s.vm.NewGoError(err))
		}
		go func() {
			response, executeErr := httpClient.Do(request)
			if err != nil {
				_, err = callback(nil, s.vm.NewGoError(executeErr), nil, nil)
				if err != nil {
					s.errorHandler(err)
				}
				return
			}
			defer response.Body.Close()
			var content []byte
			content, err = io.ReadAll(response.Body)
			if err != nil {
				_, err = callback(nil, s.vm.NewGoError(err), nil, nil)
				if err != nil {
					s.errorHandler(err)
				}
			}
			responseObject := s.vm.NewObject()
			responseObject.Set("status", response.StatusCode)
			responseObject.Set("headers", jsc.HeadersToValue(s.vm, response.Header))
			var bodyValue goja.Value
			if binaryMode {
				bodyValue = jsc.NewUint8Array(s.vm, content)
			} else {
				bodyValue = s.vm.ToValue(string(content))
			}
			_, err = callback(nil, nil, responseObject, bodyValue)
		}()
		return goja.Undefined()
	}
}
