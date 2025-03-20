package surge

import (
	"bytes"
	"compress/gzip"
	"io"

	"github.com/sagernet/sing-box/script/jsc"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/dop251/goja"
)

type Utils struct {
	class jsc.Class[*Module, *Utils]
}

func createUtils(module *Module) jsc.Class[*Module, *Utils] {
	class := jsc.NewClass[*Module, *Utils](module)
	class.DefineMethod("geoip", (*Utils).stub)
	class.DefineMethod("ipasn", (*Utils).stub)
	class.DefineMethod("ipaso", (*Utils).stub)
	class.DefineMethod("ungzip", (*Utils).ungzip)
	class.DefineMethod("toString", (*Utils).toString)
	return class
}

func (u *Utils) stub(call goja.FunctionCall) any {
	return nil
}

func (u *Utils) ungzip(call goja.FunctionCall) any {
	if len(call.Arguments) != 1 {
		panic(u.class.Runtime().NewGoError(E.New("invalid argument")))
	}
	binary := jsc.AssertBinary(u.class.Runtime(), call.Argument(0), "binary", false)
	reader, err := gzip.NewReader(bytes.NewReader(binary))
	if err != nil {
		panic(u.class.Runtime().NewGoError(err))
	}
	binary, err = io.ReadAll(reader)
	if err != nil {
		panic(u.class.Runtime().NewGoError(err))
	}
	return jsc.NewUint8Array(u.class.Runtime(), binary)
}

func (u *Utils) toString(call goja.FunctionCall) any {
	return "[sing-box Surge utils]"
}
