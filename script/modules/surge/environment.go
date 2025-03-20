package surge

import (
	"runtime"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/locale"
	"github.com/sagernet/sing-box/script/jsc"

	"github.com/dop251/goja"
)

type Environment struct {
	class jsc.Class[*Module, *Environment]
}

func createEnvironment(module *Module) jsc.Class[*Module, *Environment] {
	class := jsc.NewClass[*Module, *Environment](module)
	class.DefineField("system", (*Environment).getSystem, nil)
	class.DefineField("surge-build", (*Environment).getSurgeBuild, nil)
	class.DefineField("surge-version", (*Environment).getSurgeVersion, nil)
	class.DefineField("language", (*Environment).getLanguage, nil)
	class.DefineField("device-model", (*Environment).getDeviceModel, nil)
	class.DefineMethod("toString", (*Environment).toString)
	return class
}

func (e *Environment) getSystem() any {
	switch runtime.GOOS {
	case "ios":
		return "iOS"
	case "darwin":
		return "macOS"
	case "tvos":
		return "tvOS"
	case "linux":
		return "Linux"
	case "android":
		return "Android"
	case "windows":
		return "Windows"
	default:
		return runtime.GOOS
	}
}

func (e *Environment) getSurgeBuild() any {
	return "N/A"
}

func (e *Environment) getSurgeVersion() any {
	return "sing-box " + C.Version
}

func (e *Environment) getLanguage() any {
	return locale.Current().Locale
}

func (e *Environment) getDeviceModel() any {
	return "N/A"
}

func (e *Environment) toString(call goja.FunctionCall) any {
	return "[sing-box Surge environment"
}
