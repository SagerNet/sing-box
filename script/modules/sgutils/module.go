package sgutils

import (
	"bytes"
	"compress/gzip"
	"io"

	"github.com/sagernet/sing-box/script/jsc"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/dop251/goja"
)

type SurgeUtils struct {
	vm *goja.Runtime
}

func Enable(runtime *goja.Runtime) {
	utils := &SurgeUtils{runtime}
	object := runtime.NewObject()
	object.Set("geoip", utils.js_stub)
	object.Set("ipasn", utils.js_stub)
	object.Set("ipaso", utils.js_stub)
	object.Set("ungzip", utils.js_ungzip)
}

func (u *SurgeUtils) js_stub(call goja.FunctionCall) goja.Value {
	panic(u.vm.NewGoError(E.New("not implemented")))
}

func (u *SurgeUtils) js_ungzip(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) != 1 {
		panic(u.vm.NewGoError(E.New("invalid argument")))
	}
	binary := jsc.AssertBinary(u.vm, call.Argument(0), "binary", false)
	reader, err := gzip.NewReader(bytes.NewReader(binary))
	if err != nil {
		panic(u.vm.NewGoError(err))
	}
	binary, err = io.ReadAll(reader)
	if err != nil {
		panic(u.vm.NewGoError(err))
	}
	return jsc.NewUint8Array(u.vm, binary)
}
