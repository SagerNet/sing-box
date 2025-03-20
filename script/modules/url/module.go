package url

import (
	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing-box/script/modules/require"

	"github.com/dop251/goja"
)

const ModuleName = "url"

var _ jsc.Module = (*Module)(nil)

type Module struct {
	runtime                      *goja.Runtime
	classURL                     jsc.Class[*Module, *URL]
	classURLSearchParams         jsc.Class[*Module, *URLSearchParams]
	classURLSearchParamsIterator jsc.Class[*Module, *jsc.Iterator[*Module, searchParam]]
}

func Require(runtime *goja.Runtime, module *goja.Object) {
	m := &Module{
		runtime: runtime,
	}
	m.classURL = createURL(m)
	m.classURLSearchParams = createURLSearchParams(m)
	m.classURLSearchParamsIterator = jsc.CreateIterator[*Module, searchParam](m)
	exports := module.Get("exports").(*goja.Object)
	exports.Set("URL", m.classURL.ToValue())
	exports.Set("URLSearchParams", m.classURLSearchParams.ToValue())
}

func Enable(runtime *goja.Runtime) {
	exports := require.Require(runtime, ModuleName).ToObject(runtime)
	runtime.Set("URL", exports.Get("URL"))
	runtime.Set("URLSearchParams", exports.Get("URLSearchParams"))
}

func (m *Module) Runtime() *goja.Runtime {
	return m.runtime
}
