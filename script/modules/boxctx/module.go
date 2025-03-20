package boxctx

import (
	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing-box/script/modules/require"

	"github.com/dop251/goja"
)

const ModuleName = "context"

type Module struct {
	runtime      *goja.Runtime
	classContext jsc.Class[*Module, *Context]
}

func Require(runtime *goja.Runtime, module *goja.Object) {
	m := &Module{
		runtime: runtime,
	}
	m.classContext = createContext(m)
	exports := module.Get("exports").(*goja.Object)
	exports.Set("Context", m.classContext.ToValue())
}

func Enable(runtime *goja.Runtime, context *Context) {
	exports := require.Require(runtime, ModuleName).ToObject(runtime)
	classContext := jsc.GetClass[*Module, *Context](runtime, exports, "Context")
	context.class = classContext
	runtime.Set("context", classContext.New(context))
}

func (m *Module) Runtime() *goja.Runtime {
	return m.runtime
}
