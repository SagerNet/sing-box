package jsc

import (
	"net/http"
	"reflect"

	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"

	"github.com/dop251/goja"
)

func HeadersToValue(runtime *goja.Runtime, headers http.Header) goja.Value {
	object := runtime.NewObject()
	for key, value := range headers {
		if len(value) == 1 {
			object.Set(key, value[0])
		} else {
			object.Set(key, ArrayToValue(runtime, value))
		}
	}
	return object
}

func ArrayToValue[T any](runtime *goja.Runtime, values []T) goja.Value {
	return runtime.NewArray(common.Map(values, func(it T) any { return it })...)
}

func ObjectToHeaders(vm *goja.Runtime, object *goja.Object, name string) http.Header {
	headers := make(http.Header)
	for _, key := range object.Keys() {
		valueObject := object.Get(key)
		switch headerValue := valueObject.(type) {
		case goja.String:
			headers.Set(key, headerValue.String())
		case *goja.Object:
			values := headerValue.Export()
			valueArray, isArray := values.([]any)
			if !isArray {
				panic(vm.NewTypeError(F.ToString("invalid value: ", name, ".", key, "expected string or string array, got ", valueObject.String())))
			}
			newValues := make([]string, 0, len(valueArray))
			for _, value := range valueArray {
				stringValue, isString := value.(string)
				if !isString {
					panic(vm.NewTypeError(F.ToString("invalid value: ", name, ".", key, " expected string or string array, got array item type: ", reflect.TypeOf(value))))
				}
				newValues = append(newValues, stringValue)
			}
			headers[key] = newValues
		default:
			panic(vm.NewTypeError(F.ToString("invalid value: ", name, ".", key, " expected string or string array, got ", valueObject.String())))
		}
	}
	return headers
}
