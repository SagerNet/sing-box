package jsc

import (
	"net/http"

	F "github.com/sagernet/sing/common/format"

	"github.com/dop251/goja"
)

func IsNil(value goja.Value) bool {
	return value == nil || goja.IsUndefined(value) || goja.IsNull(value)
}

func AssertObject(vm *goja.Runtime, value goja.Value, name string, nilable bool) *goja.Object {
	if IsNil(value) {
		if nilable {
			return nil
		}
		panic(vm.NewTypeError(F.ToString("invalid argument: missing ", name)))
	}
	objectValue, isObject := value.(*goja.Object)
	if !isObject {
		panic(vm.NewTypeError(F.ToString("invalid argument: ", name, ": expected object, but got ", value)))
	}
	return objectValue
}

func AssertString(vm *goja.Runtime, value goja.Value, name string, nilable bool) string {
	if IsNil(value) {
		if nilable {
			return ""
		}
		panic(vm.NewTypeError(F.ToString("invalid argument: missing ", name)))
	}
	stringValue, isString := value.Export().(string)
	if !isString {
		panic(vm.NewTypeError(F.ToString("invalid argument: ", name, ": expected string, but got ", value)))
	}
	return stringValue
}

func AssertInt(vm *goja.Runtime, value goja.Value, name string, nilable bool) int64 {
	if IsNil(value) {
		if nilable {
			return 0
		}
		panic(vm.NewTypeError(F.ToString("invalid argument: missing ", name)))
	}
	integerValue, isNumber := value.Export().(int64)
	if !isNumber {
		panic(vm.NewTypeError(F.ToString("invalid argument: ", name, ": expected integer, but got ", value)))
	}
	return integerValue
}

func AssertBool(vm *goja.Runtime, value goja.Value, name string, nilable bool) bool {
	if IsNil(value) {
		if nilable {
			return false
		}
		panic(vm.NewTypeError(F.ToString("invalid argument: missing ", name)))
	}
	boolValue, isBool := value.Export().(bool)
	if !isBool {
		panic(vm.NewTypeError(F.ToString("invalid argument: ", name, ": expected boolean, but got ", value)))
	}
	return boolValue
}

func AssertBinary(vm *goja.Runtime, value goja.Value, name string, nilable bool) []byte {
	if IsNil(value) {
		if nilable {
			return nil
		}
		panic(vm.NewTypeError(F.ToString("invalid argument: missing ", name)))
	}
	switch exportedValue := value.Export().(type) {
	case []byte:
		return exportedValue
	case goja.ArrayBuffer:
		return exportedValue.Bytes()
	default:
		panic(vm.NewTypeError(F.ToString("invalid argument: ", name, ": expected Uint8Array or ArrayBuffer, but got ", value)))
	}
}

func AssertStringBinary(vm *goja.Runtime, value goja.Value, name string, nilable bool) []byte {
	if IsNil(value) {
		if nilable {
			return nil
		}
		panic(vm.NewTypeError(F.ToString("invalid argument: missing ", name)))
	}
	switch exportedValue := value.Export().(type) {
	case string:
		return []byte(exportedValue)
	case []byte:
		return exportedValue
	case goja.ArrayBuffer:
		return exportedValue.Bytes()
	default:
		panic(vm.NewTypeError(F.ToString("invalid argument: ", name, ": expected string, Uint8Array or ArrayBuffer, but got ", value)))
	}
}

func AssertFunction(vm *goja.Runtime, value goja.Value, name string) goja.Callable {
	functionValue, isFunction := goja.AssertFunction(value)
	if !isFunction {
		panic(vm.NewTypeError(F.ToString("invalid argument: ", name, ": expected function, but got ", value)))
	}
	return functionValue
}

func AssertHTTPHeader(vm *goja.Runtime, value goja.Value, name string) http.Header {
	headersObject := AssertObject(vm, value, name, true)
	if headersObject == nil {
		return nil
	}
	return ObjectToHeaders(vm, headersObject, name)
}
