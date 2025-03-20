package url

import (
	"net"
	"net/url"
	"strings"

	"github.com/sagernet/sing-box/script/jsc"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/dop251/goja"
	"golang.org/x/net/idna"
)

type URL struct {
	class       jsc.Class[*Module, *URL]
	url         *url.URL
	params      *URLSearchParams
	paramsValue goja.Value
}

func newURL(c jsc.Class[*Module, *URL], call goja.ConstructorCall) *URL {
	var (
		u, base *url.URL
		err     error
	)
	switch argURL := call.Argument(0).Export().(type) {
	case *URL:
		u = argURL.url
	default:
		u, err = parseURL(call.Argument(0).String())
		if err != nil {
			panic(c.Runtime().NewGoError(E.Cause(err, "parse URL")))
		}
	}
	if len(call.Arguments) == 2 {
		switch argBaseURL := call.Argument(1).Export().(type) {
		case *URL:
			base = argBaseURL.url
		default:
			base, err = parseURL(call.Argument(1).String())
			if err != nil {
				panic(c.Runtime().NewGoError(E.Cause(err, "parse base URL")))
			}
		}
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	return &URL{class: c, url: u}
}

func createURL(module *Module) jsc.Class[*Module, *URL] {
	class := jsc.NewClass[*Module, *URL](module)
	class.DefineConstructor(newURL)
	class.DefineField("hash", (*URL).getHash, (*URL).setHash)
	class.DefineField("host", (*URL).getHost, (*URL).setHost)
	class.DefineField("hostname", (*URL).getHostName, (*URL).setHostName)
	class.DefineField("href", (*URL).getHref, (*URL).setHref)
	class.DefineField("origin", (*URL).getOrigin, nil)
	class.DefineField("password", (*URL).getPassword, (*URL).setPassword)
	class.DefineField("pathname", (*URL).getPathname, (*URL).setPathname)
	class.DefineField("port", (*URL).getPort, (*URL).setPort)
	class.DefineField("protocol", (*URL).getProtocol, (*URL).setProtocol)
	class.DefineField("search", (*URL).getSearch, (*URL).setSearch)
	class.DefineField("searchParams", (*URL).getSearchParams, (*URL).setSearchParams)
	class.DefineField("username", (*URL).getUsername, (*URL).setUsername)
	class.DefineMethod("toString", (*URL).toString)
	class.DefineMethod("toJSON", (*URL).toJSON)
	class.DefineStaticMethod("canParse", canParse)
	// class.DefineStaticMethod("createObjectURL", createObjectURL)
	class.DefineStaticMethod("parse", parse)
	// class.DefineStaticMethod("revokeObjectURL", revokeObjectURL)
	return class
}

func canParse(class jsc.Class[*Module, *URL], call goja.FunctionCall) any {
	switch call.Argument(0).Export().(type) {
	case *URL:
	default:
		_, err := parseURL(call.Argument(0).String())
		if err != nil {
			return false
		}
	}
	if len(call.Arguments) == 2 {
		switch call.Argument(1).Export().(type) {
		case *URL:
		default:
			_, err := parseURL(call.Argument(1).String())
			if err != nil {
				return false
			}
		}
	}
	return true
}

func parse(class jsc.Class[*Module, *URL], call goja.FunctionCall) any {
	var (
		u, base *url.URL
		err     error
	)
	switch argURL := call.Argument(0).Export().(type) {
	case *URL:
		u = argURL.url
	default:
		u, err = parseURL(call.Argument(0).String())
		if err != nil {
			return goja.Null()
		}
	}
	if len(call.Arguments) == 2 {
		switch argBaseURL := call.Argument(1).Export().(type) {
		case *URL:
			base = argBaseURL.url
		default:
			base, err = parseURL(call.Argument(1).String())
			if err != nil {
				return goja.Null()
			}
		}
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	return &URL{class: class, url: u}
}

func (r *URL) getHash() any {
	if r.url.Fragment != "" {
		return "#" + r.url.EscapedFragment()
	}
	return ""
}

func (r *URL) setHash(value goja.Value) {
	r.url.RawFragment = strings.TrimPrefix(value.String(), "#")
}

func (r *URL) getHost() any {
	return r.url.Host
}

func (r *URL) setHost(value goja.Value) {
	r.url.Host = strings.TrimSuffix(value.String(), ":")
}

func (r *URL) getHostName() any {
	return r.url.Hostname()
}

func (r *URL) setHostName(value goja.Value) {
	r.url.Host = joinHostPort(value.String(), r.url.Port())
}

func (r *URL) getHref() any {
	return r.url.String()
}

func (r *URL) setHref(value goja.Value) {
	newURL, err := url.Parse(value.String())
	if err != nil {
		panic(r.class.Runtime().NewGoError(err))
	}
	r.url = newURL
	r.params = nil
}

func (r *URL) getOrigin() any {
	return r.url.Scheme + "://" + r.url.Host
}

func (r *URL) getPassword() any {
	if r.url.User != nil {
		password, _ := r.url.User.Password()
		return password
	}
	return ""
}

func (r *URL) setPassword(value goja.Value) {
	if r.url.User == nil {
		r.url.User = url.UserPassword("", value.String())
	} else {
		r.url.User = url.UserPassword(r.url.User.Username(), value.String())
	}
}

func (r *URL) getPathname() any {
	return r.url.EscapedPath()
}

func (r *URL) setPathname(value goja.Value) {
	r.url.RawPath = value.String()
}

func (r *URL) getPort() any {
	return r.url.Port()
}

func (r *URL) setPort(value goja.Value) {
	r.url.Host = joinHostPort(r.url.Hostname(), value.String())
}

func (r *URL) getProtocol() any {
	return r.url.Scheme + ":"
}

func (r *URL) setProtocol(value goja.Value) {
	r.url.Scheme = strings.TrimSuffix(value.String(), ":")
}

func (r *URL) getSearch() any {
	if r.params != nil {
		if len(r.params.params) > 0 {
			return "?" + generateQuery(r.params.params)
		}
	} else if r.url.RawQuery != "" {
		return "?" + r.url.RawQuery
	}
	return ""
}

func (r *URL) setSearch(value goja.Value) {
	params, err := parseQuery(value.String())
	if err == nil {
		if r.params != nil {
			r.params.params = params
		} else {
			r.url.RawQuery = generateQuery(params)
		}
	}
}

func (r *URL) getSearchParams() any {
	var params []searchParam
	if r.url.RawQuery != "" {
		params, _ = parseQuery(r.url.RawQuery)
	}
	if r.params == nil {
		r.params = &URLSearchParams{
			class:  r.class.Module().classURLSearchParams,
			params: params,
		}
		r.paramsValue = r.class.Module().classURLSearchParams.New(r.params)
	}
	return r.paramsValue
}

func (r *URL) setSearchParams(value goja.Value) {
	if params, ok := value.Export().(*URLSearchParams); ok {
		r.params = params
		r.paramsValue = value
	}
}

func (r *URL) getUsername() any {
	if r.url.User != nil {
		return r.url.User.Username()
	}
	return ""
}

func (r *URL) setUsername(value goja.Value) {
	if r.url.User == nil {
		r.url.User = url.User(value.String())
	} else {
		password, _ := r.url.User.Password()
		r.url.User = url.UserPassword(value.String(), password)
	}
}

func (r *URL) toString(call goja.FunctionCall) any {
	if r.params != nil {
		r.url.RawQuery = generateQuery(r.params.params)
	}
	return r.url.String()
}

func (r *URL) toJSON(call goja.FunctionCall) any {
	return r.toString(call)
}

func parseURL(s string) (*url.URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, E.Cause(err, "invalid URL")
	}
	switch u.Scheme {
	case "https", "http", "ftp", "wss", "ws":
		if u.Path == "" {
			u.Path = "/"
		}
		hostname := u.Hostname()
		asciiHostname, err := idna.Punycode.ToASCII(strings.ToLower(hostname))
		if err != nil {
			return nil, E.Cause(err, "invalid hostname")
		}
		if asciiHostname != hostname {
			u.Host = joinHostPort(asciiHostname, u.Port())
		}
	}
	if u.RawQuery != "" {
		u.RawQuery = escape(u.RawQuery, &tblEscapeURLQuery, false)
	}
	return u, nil
}

func joinHostPort(hostname, port string) string {
	if port == "" {
		return hostname
	}
	return net.JoinHostPort(hostname, port)
}
