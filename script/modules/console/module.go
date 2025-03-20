package console

import (
	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing-box/script/modules/require"

	"github.com/dop251/goja"
)

const ModuleName = "console"

type Module struct {
	runtime *goja.Runtime
	console jsc.Class[*Module, *Console]
}

func Require(runtime *goja.Runtime, module *goja.Object) {
	m := &Module{
		runtime: runtime,
	}
	m.console = createConsole(m)
	exports := module.Get("exports").(*goja.Object)
	exports.Set("Console", m.console.ToValue())
}

func Enable(runtime *goja.Runtime) {
	exports := require.Require(runtime, ModuleName).ToObject(runtime)
	classConsole := jsc.GetClass[*Module, *Console](runtime, exports, "Console")
	runtime.Set("console", NewConsole(classConsole))
}

func (m *Module) Runtime() *goja.Runtime {
	return m.runtime
}
