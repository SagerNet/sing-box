package boxctx

import (
	"context"
	"time"

	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing/common/logger"

	"github.com/dop251/goja"
)

type Context struct {
	class        jsc.Class[*Module, *Context]
	Context      context.Context
	Logger       logger.ContextLogger
	Tag          string
	StartedAt    time.Time
	ErrorHandler func(error)
}

func FromRuntime(runtime *goja.Runtime) *Context {
	contextValue := runtime.Get("context")
	if contextValue == nil {
		return nil
	}
	context, isContext := contextValue.Export().(*Context)
	if !isContext {
		return nil
	}
	return context
}

func MustFromRuntime(runtime *goja.Runtime) *Context {
	context := FromRuntime(runtime)
	if context == nil {
		panic(runtime.NewTypeError("Missing sing-box context"))
	}
	return context
}

func createContext(module *Module) jsc.Class[*Module, *Context] {
	class := jsc.NewClass[*Module, *Context](module)
	class.DefineMethod("toString", (*Context).toString)
	return class
}

func (c *Context) toString(call goja.FunctionCall) any {
	return "[sing-box Context]"
}
