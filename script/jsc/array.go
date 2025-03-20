package jsc

import (
	_ "unsafe"

	"github.com/dop251/goja"
)

func NewUint8Array(runtime *goja.Runtime, data []byte) goja.Value {
	buffer := runtime.NewArrayBuffer(data)
	ctor, loaded := goja.AssertConstructor(runtimeGetUint8Array(runtime))
	if !loaded {
		panic(runtime.NewTypeError("missing UInt8Array constructor"))
	}
	array, err := ctor(nil, runtime.ToValue(buffer))
	if err != nil {
		panic(runtime.NewGoError(err))
	}
	return array
}

//go:linkname runtimeGetUint8Array github.com/dop251/goja.(*Runtime).getUint8Array
func runtimeGetUint8Array(r *goja.Runtime) *goja.Object
