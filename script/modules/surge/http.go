package surge

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"

	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing-box/script/modules/boxctx"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"

	"github.com/dop251/goja"
	"golang.org/x/net/publicsuffix"
)

type HTTP struct {
	class         jsc.Class[*Module, *HTTP]
	cookieJar     *cookiejar.Jar
	httpTransport *http.Transport
}

func createHTTP(module *Module) jsc.Class[*Module, *HTTP] {
	class := jsc.NewClass[*Module, *HTTP](module)
	class.DefineConstructor(newHTTP)
	class.DefineMethod("get", httpRequest(http.MethodGet))
	class.DefineMethod("post", httpRequest(http.MethodPost))
	class.DefineMethod("put", httpRequest(http.MethodPut))
	class.DefineMethod("delete", httpRequest(http.MethodDelete))
	class.DefineMethod("head", httpRequest(http.MethodHead))
	class.DefineMethod("options", httpRequest(http.MethodOptions))
	class.DefineMethod("patch", httpRequest(http.MethodPatch))
	class.DefineMethod("trace", httpRequest(http.MethodTrace))
	class.DefineMethod("toString", (*HTTP).toString)
	return class
}

func newHTTP(class jsc.Class[*Module, *HTTP], call goja.ConstructorCall) *HTTP {
	return &HTTP{
		class: class,
		cookieJar: common.Must1(cookiejar.New(&cookiejar.Options{
			PublicSuffixList: publicsuffix.List,
		})),
		httpTransport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   &tls.Config{},
		},
	}
}

func httpRequest(method string) func(s *HTTP, call goja.FunctionCall) any {
	return func(s *HTTP, call goja.FunctionCall) any {
		if len(call.Arguments) != 2 {
			panic(s.class.Runtime().NewTypeError("invalid arguments"))
		}
		context := boxctx.MustFromRuntime(s.class.Runtime())
		var (
			url          string
			headers      http.Header
			body         []byte
			timeout      = 5 * time.Second
			insecure     bool
			autoCookie   bool = true
			autoRedirect bool
			// policy       string
			binaryMode bool
		)
		switch optionsValue := call.Argument(0).(type) {
		case goja.String:
			url = optionsValue.String()
		case *goja.Object:
			url = jsc.AssertString(s.class.Runtime(), optionsValue.Get("url"), "options.url", false)
			headers = jsc.AssertHTTPHeader(s.class.Runtime(), optionsValue.Get("headers"), "option.headers")
			body = jsc.AssertStringBinary(s.class.Runtime(), optionsValue.Get("body"), "options.body", true)
			timeoutInt := jsc.AssertInt(s.class.Runtime(), optionsValue.Get("timeout"), "options.timeout", true)
			if timeoutInt > 0 {
				timeout = time.Duration(timeoutInt) * time.Second
			}
			insecure = jsc.AssertBool(s.class.Runtime(), optionsValue.Get("insecure"), "options.insecure", true)
			autoCookie = jsc.AssertBool(s.class.Runtime(), optionsValue.Get("auto-cookie"), "options.auto-cookie", true)
			autoRedirect = jsc.AssertBool(s.class.Runtime(), optionsValue.Get("auto-redirect"), "options.auto-redirect", true)
			// policy = jsc.AssertString(s.class.Runtime(), optionsValue.Get("policy"), "options.policy", true)
			binaryMode = jsc.AssertBool(s.class.Runtime(), optionsValue.Get("binary-mode"), "options.binary-mode", true)
		default:
			panic(s.class.Runtime().NewTypeError(F.ToString("invalid argument: options: expected string or object, but got ", optionsValue)))
		}
		callback := jsc.AssertFunction(s.class.Runtime(), call.Argument(1), "callback")
		s.httpTransport.TLSClientConfig.InsecureSkipVerify = insecure
		httpClient := &http.Client{
			Timeout:   timeout,
			Transport: s.httpTransport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if autoRedirect {
					return nil
				}
				return http.ErrUseLastResponse
			},
		}
		if autoCookie {
			httpClient.Jar = s.cookieJar
		}
		request, err := http.NewRequestWithContext(context.Context, method, url, bytes.NewReader(body))
		if host := headers.Get("Host"); host != "" {
			request.Host = host
			headers.Del("Host")
		}
		request.Header = headers
		if err != nil {
			panic(s.class.Runtime().NewGoError(err))
		}
		go func() {
			defer s.httpTransport.CloseIdleConnections()
			response, executeErr := httpClient.Do(request)
			if err != nil {
				_, err = callback(nil, s.class.Runtime().NewGoError(executeErr), nil, nil)
				if err != nil {
					context.ErrorHandler(err)
				}
				return
			}
			defer response.Body.Close()
			var content []byte
			content, err = io.ReadAll(response.Body)
			if err != nil {
				_, err = callback(nil, s.class.Runtime().NewGoError(err), nil, nil)
				if err != nil {
					context.ErrorHandler(err)
				}
			}
			responseObject := s.class.Runtime().NewObject()
			responseObject.Set("status", response.StatusCode)
			responseObject.Set("headers", jsc.HeadersToValue(s.class.Runtime(), response.Header))
			var bodyValue goja.Value
			if binaryMode {
				bodyValue = jsc.NewUint8Array(s.class.Runtime(), content)
			} else {
				bodyValue = s.class.Runtime().ToValue(string(content))
			}
			_, err = callback(nil, nil, responseObject, bodyValue)
		}()
		return nil
	}
}

func (h *HTTP) toString(call goja.FunctionCall) any {
	return "[sing-box Surge HTTP]"
}
