package surge

import (
	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing-box/script/modules/require"
	"github.com/sagernet/sing/common"

	"github.com/dop251/goja"
)

const ModuleName = "surge"

type Module struct {
	runtime              *goja.Runtime
	classScript          jsc.Class[*Module, *Script]
	classEnvironment     jsc.Class[*Module, *Environment]
	classPersistentStore jsc.Class[*Module, *PersistentStore]
	classHTTP            jsc.Class[*Module, *HTTP]
	classUtils           jsc.Class[*Module, *Utils]
	classNotification    jsc.Class[*Module, *Notification]
}

func Require(runtime *goja.Runtime, module *goja.Object) {
	m := &Module{
		runtime: runtime,
	}
	m.classScript = createScript(m)
	m.classEnvironment = createEnvironment(m)
	m.classPersistentStore = createPersistentStore(m)
	m.classHTTP = createHTTP(m)
	m.classUtils = createUtils(m)
	m.classNotification = createNotification(m)
	exports := module.Get("exports").(*goja.Object)
	exports.Set("Script", m.classScript.ToValue())
	exports.Set("Environment", m.classEnvironment.ToValue())
	exports.Set("PersistentStore", m.classPersistentStore.ToValue())
	exports.Set("HTTP", m.classHTTP.ToValue())
	exports.Set("Utils", m.classUtils.ToValue())
	exports.Set("Notification", m.classNotification.ToValue())
}

func Enable(runtime *goja.Runtime, scriptType string, args []string) {
	exports := require.Require(runtime, ModuleName).ToObject(runtime)
	classScript := jsc.GetClass[*Module, *Script](runtime, exports, "Script")
	classEnvironment := jsc.GetClass[*Module, *Environment](runtime, exports, "Environment")
	classPersistentStore := jsc.GetClass[*Module, *PersistentStore](runtime, exports, "PersistentStore")
	classHTTP := jsc.GetClass[*Module, *HTTP](runtime, exports, "HTTP")
	classUtils := jsc.GetClass[*Module, *Utils](runtime, exports, "Utils")
	classNotification := jsc.GetClass[*Module, *Notification](runtime, exports, "Notification")
	runtime.Set("$script", classScript.New(&Script{class: classScript, ScriptType: scriptType}))
	runtime.Set("$environment", classEnvironment.New(&Environment{class: classEnvironment}))
	runtime.Set("$persistentStore", newPersistentStore(classPersistentStore))
	runtime.Set("$http", classHTTP.New(newHTTP(classHTTP, goja.ConstructorCall{})))
	runtime.Set("$utils", classUtils.New(&Utils{class: classUtils}))
	runtime.Set("$notification", newNotification(classNotification))
	runtime.Set("$argument", runtime.NewArray(common.Map(args, func(it string) any {
		return it
	})...))
}

func (m *Module) Runtime() *goja.Runtime {
	return m.runtime
}
