package url

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/sagernet/sing-box/script/jsc"
	F "github.com/sagernet/sing/common/format"

	"github.com/dop251/goja"
)

type URLSearchParams struct {
	class  jsc.Class[*Module, *URLSearchParams]
	params []searchParam
}

func createURLSearchParams(module *Module) jsc.Class[*Module, *URLSearchParams] {
	class := jsc.NewClass[*Module, *URLSearchParams](module)
	class.DefineConstructor(newURLSearchParams)
	class.DefineField("size", (*URLSearchParams).getSize, nil)
	class.DefineMethod("append", (*URLSearchParams).append)
	class.DefineMethod("delete", (*URLSearchParams).delete)
	class.DefineMethod("entries", (*URLSearchParams).entries)
	class.DefineMethod("forEach", (*URLSearchParams).forEach)
	class.DefineMethod("get", (*URLSearchParams).get)
	class.DefineMethod("getAll", (*URLSearchParams).getAll)
	class.DefineMethod("has", (*URLSearchParams).has)
	class.DefineMethod("keys", (*URLSearchParams).keys)
	class.DefineMethod("set", (*URLSearchParams).set)
	class.DefineMethod("sort", (*URLSearchParams).sort)
	class.DefineMethod("toString", (*URLSearchParams).toString)
	class.DefineMethod("values", (*URLSearchParams).values)
	return class
}

func newURLSearchParams(class jsc.Class[*Module, *URLSearchParams], call goja.ConstructorCall) *URLSearchParams {
	var (
		params []searchParam
		err    error
	)
	switch argInit := call.Argument(0).Export().(type) {
	case *URLSearchParams:
		params = argInit.params
	case string:
		params, err = parseQuery(argInit)
		if err != nil {
			panic(class.Runtime().NewGoError(err))
		}
	case [][]string:
		for _, pair := range argInit {
			if len(pair) != 2 {
				panic(class.Runtime().NewTypeError("Each query pair must be an iterable [name, value] tuple"))
			}
			params = append(params, searchParam{pair[0], pair[1]})
		}
	case map[string]any:
		for name, value := range argInit {
			stringValue, isString := value.(string)
			if !isString {
				panic(class.Runtime().NewTypeError("Invalid query value"))
			}
			params = append(params, searchParam{name, stringValue})
		}
	}
	return &URLSearchParams{class, params}
}

func (s *URLSearchParams) getSize() any {
	return len(s.params)
}

func (s *URLSearchParams) append(call goja.FunctionCall) any {
	name := jsc.AssertString(s.class.Runtime(), call.Argument(0), "name", false)
	value := call.Argument(1).String()
	s.params = append(s.params, searchParam{name, value})
	return goja.Undefined()
}

func (s *URLSearchParams) delete(call goja.FunctionCall) any {
	name := jsc.AssertString(s.class.Runtime(), call.Argument(0), "name", false)
	argValue := call.Argument(1)
	if !jsc.IsNil(argValue) {
		value := argValue.String()
		for i, param := range s.params {
			if param.Key == name && param.Value == value {
				s.params = append(s.params[:i], s.params[i+1:]...)
				break
			}
		}
	} else {
		for i, param := range s.params {
			if param.Key == name {
				s.params = append(s.params[:i], s.params[i+1:]...)
				break
			}
		}
	}
	return goja.Undefined()
}

func (s *URLSearchParams) entries(call goja.FunctionCall) any {
	return jsc.NewIterator[*Module, searchParam](s.class.Module().classURLSearchParamsIterator, s.params, func(this searchParam) any {
		return s.class.Runtime().NewArray(this.Key, this.Value)
	})
}

func (s *URLSearchParams) forEach(call goja.FunctionCall) any {
	callback := jsc.AssertFunction(s.class.Runtime(), call.Argument(0), "callbackFn")
	thisValue := call.Argument(1)
	for _, param := range s.params {
		for _, value := range param.Value {
			_, err := callback(thisValue, s.class.Runtime().ToValue(value), s.class.Runtime().ToValue(param.Key), call.This)
			if err != nil {
				panic(s.class.Runtime().NewGoError(err))
			}
		}
	}
	return goja.Undefined()
}

func (s *URLSearchParams) get(call goja.FunctionCall) any {
	name := jsc.AssertString(s.class.Runtime(), call.Argument(0), "name", false)
	for _, param := range s.params {
		if param.Key == name {
			return param.Value
		}
	}
	return goja.Null()
}

func (s *URLSearchParams) getAll(call goja.FunctionCall) any {
	name := jsc.AssertString(s.class.Runtime(), call.Argument(0), "name", false)
	var values []any
	for _, param := range s.params {
		if param.Key == name {
			values = append(values, param.Value)
		}
	}
	return s.class.Runtime().NewArray(values...)
}

func (s *URLSearchParams) has(call goja.FunctionCall) any {
	name := jsc.AssertString(s.class.Runtime(), call.Argument(0), "name", false)
	argValue := call.Argument(1)
	if !jsc.IsNil(argValue) {
		value := argValue.String()
		for _, param := range s.params {
			if param.Key == name && param.Value == value {
				return true
			}
		}
	} else {
		for _, param := range s.params {
			if param.Key == name {
				return true
			}
		}
	}
	return false
}

func (s *URLSearchParams) keys(call goja.FunctionCall) any {
	return jsc.NewIterator[*Module, searchParam](s.class.Module().classURLSearchParamsIterator, s.params, func(this searchParam) any {
		return this.Key
	})
}

func (s *URLSearchParams) set(call goja.FunctionCall) any {
	name := jsc.AssertString(s.class.Runtime(), call.Argument(0), "name", false)
	value := call.Argument(1).String()
	for i, param := range s.params {
		if param.Key == name {
			s.params[i].Value = value
			return goja.Undefined()
		}
	}
	s.params = append(s.params, searchParam{name, value})
	return goja.Undefined()
}

func (s *URLSearchParams) sort(call goja.FunctionCall) any {
	sort.SliceStable(s.params, func(i, j int) bool {
		return s.params[i].Key < s.params[j].Key
	})
	return goja.Undefined()
}

func (s *URLSearchParams) toString(call goja.FunctionCall) any {
	return generateQuery(s.params)
}

func (s *URLSearchParams) values(call goja.FunctionCall) any {
	return jsc.NewIterator[*Module, searchParam](s.class.Module().classURLSearchParamsIterator, s.params, func(this searchParam) any {
		return this.Value
	})
}

type searchParam struct {
	Key   string
	Value string
}

func parseQuery(query string) (params []searchParam, err error) {
	query = strings.TrimPrefix(query, "?")
	for query != "" {
		var key string
		key, query, _ = strings.Cut(query, "&")
		if strings.Contains(key, ";") {
			err = fmt.Errorf("invalid semicolon separator in query")
			continue
		}
		if key == "" {
			continue
		}
		key, value, _ := strings.Cut(key, "=")
		key, err1 := url.QueryUnescape(key)
		if err1 != nil {
			if err == nil {
				err = err1
			}
			continue
		}
		value, err1 = url.QueryUnescape(value)
		if err1 != nil {
			if err == nil {
				err = err1
			}
			continue
		}
		params = append(params, searchParam{key, value})
	}
	return
}

func generateQuery(params []searchParam) string {
	var parts []string
	for _, param := range params {
		parts = append(parts, F.ToString(param.Key, "=", url.QueryEscape(param.Value)))
	}
	return strings.Join(parts, "&")
}
