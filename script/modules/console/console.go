package console

import (
	"bytes"
	"context"
	"encoding/xml"
	"sync"
	"time"

	sLog "github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing-box/script/modules/boxctx"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"

	"github.com/dop251/goja"
)

type Console struct {
	class    jsc.Class[*Module, *Console]
	access   sync.Mutex
	countMap map[string]int
	timeMap  map[string]time.Time
}

func NewConsole(class jsc.Class[*Module, *Console]) goja.Value {
	return class.New(&Console{
		class:    class,
		countMap: make(map[string]int),
		timeMap:  make(map[string]time.Time),
	})
}

func createConsole(m *Module) jsc.Class[*Module, *Console] {
	class := jsc.NewClass[*Module, *Console](m)
	class.DefineMethod("assert", (*Console).assert)
	class.DefineMethod("clear", (*Console).clear)
	class.DefineMethod("count", (*Console).count)
	class.DefineMethod("countReset", (*Console).countReset)
	class.DefineMethod("debug", (*Console).debug)
	class.DefineMethod("dir", (*Console).dir)
	class.DefineMethod("dirxml", (*Console).dirxml)
	class.DefineMethod("error", (*Console).error)
	class.DefineMethod("group", (*Console).stub)
	class.DefineMethod("groupCollapsed", (*Console).stub)
	class.DefineMethod("groupEnd", (*Console).stub)
	class.DefineMethod("info", (*Console).info)
	class.DefineMethod("log", (*Console)._log)
	class.DefineMethod("profile", (*Console).stub)
	class.DefineMethod("profileEnd", (*Console).profileEnd)
	class.DefineMethod("table", (*Console).table)
	class.DefineMethod("time", (*Console).time)
	class.DefineMethod("timeEnd", (*Console).timeEnd)
	class.DefineMethod("timeLog", (*Console).timeLog)
	class.DefineMethod("timeStamp", (*Console).stub)
	class.DefineMethod("trace", (*Console).trace)
	class.DefineMethod("warn", (*Console).warn)
	return class
}

func (c *Console) stub(call goja.FunctionCall) any {
	return goja.Undefined()
}

func (c *Console) assert(call goja.FunctionCall) any {
	assertion := call.Argument(0).ToBoolean()
	if !assertion {
		return c.log(logger.ContextLogger.ErrorContext, call.Arguments[1:])
	}
	return goja.Undefined()
}

func (c *Console) clear(call goja.FunctionCall) any {
	return nil
}

func (c *Console) count(call goja.FunctionCall) any {
	label := jsc.AssertString(c.class.Runtime(), call.Argument(0), "label", true)
	if label == "" {
		label = "default"
	}
	c.access.Lock()
	newValue := c.countMap[label] + 1
	c.countMap[label] = newValue
	c.access.Unlock()
	writeLog(c.class.Runtime(), logger.ContextLogger.InfoContext, F.ToString(label, ": ", newValue))
	return goja.Undefined()
}

func (c *Console) countReset(call goja.FunctionCall) any {
	label := jsc.AssertString(c.class.Runtime(), call.Argument(0), "label", true)
	if label == "" {
		label = "default"
	}
	c.access.Lock()
	delete(c.countMap, label)
	c.access.Unlock()
	return goja.Undefined()
}

func (c *Console) log(logFunc func(logger.ContextLogger, context.Context, ...any), args []goja.Value) any {
	var buffer bytes.Buffer
	var formatString string
	if len(args) > 0 {
		formatString = args[0].String()
	}
	format(c.class.Runtime(), &buffer, formatString, args[1:]...)
	writeLog(c.class.Runtime(), logFunc, buffer.String())
	return goja.Undefined()
}

func (c *Console) debug(call goja.FunctionCall) any {
	return c.log(logger.ContextLogger.DebugContext, call.Arguments)
}

func (c *Console) dir(call goja.FunctionCall) any {
	object := jsc.AssertObject(c.class.Runtime(), call.Argument(0), "object", false)
	var buffer bytes.Buffer
	for _, key := range object.Keys() {
		value := object.Get(key)
		buffer.WriteString(key)
		buffer.WriteString(": ")
		buffer.WriteString(value.String())
		buffer.WriteString("\n")
	}
	writeLog(c.class.Runtime(), logger.ContextLogger.InfoContext, buffer.String())
	return goja.Undefined()
}

func (c *Console) dirxml(call goja.FunctionCall) any {
	var buffer bytes.Buffer
	encoder := xml.NewEncoder(&buffer)
	encoder.Indent("", "  ")
	encoder.Encode(call.Argument(0).Export())
	writeLog(c.class.Runtime(), logger.ContextLogger.InfoContext, buffer.String())
	return goja.Undefined()
}

func (c *Console) error(call goja.FunctionCall) any {
	return c.log(logger.ContextLogger.ErrorContext, call.Arguments)
}

func (c *Console) info(call goja.FunctionCall) any {
	return c.log(logger.ContextLogger.InfoContext, call.Arguments)
}

func (c *Console) _log(call goja.FunctionCall) any {
	return c.log(logger.ContextLogger.InfoContext, call.Arguments)
}

func (c *Console) profileEnd(call goja.FunctionCall) any {
	return goja.Undefined()
}

func (c *Console) table(call goja.FunctionCall) any {
	return c.dir(call)
}

func (c *Console) time(call goja.FunctionCall) any {
	label := jsc.AssertString(c.class.Runtime(), call.Argument(0), "label", true)
	if label == "" {
		label = "default"
	}
	c.access.Lock()
	c.timeMap[label] = time.Now()
	c.access.Unlock()
	return goja.Undefined()
}

func (c *Console) timeEnd(call goja.FunctionCall) any {
	label := jsc.AssertString(c.class.Runtime(), call.Argument(0), "label", true)
	if label == "" {
		label = "default"
	}
	c.access.Lock()
	startTime, ok := c.timeMap[label]
	if !ok {
		c.access.Unlock()
		return goja.Undefined()
	}
	delete(c.timeMap, label)
	c.access.Unlock()
	writeLog(c.class.Runtime(), logger.ContextLogger.InfoContext, F.ToString(label, ": ", time.Since(startTime).String(), " - - timer ended"))
	return goja.Undefined()
}

func (c *Console) timeLog(call goja.FunctionCall) any {
	label := jsc.AssertString(c.class.Runtime(), call.Argument(0), "label", true)
	if label == "" {
		label = "default"
	}
	c.access.Lock()
	startTime, ok := c.timeMap[label]
	c.access.Unlock()
	if !ok {
		writeLog(c.class.Runtime(), logger.ContextLogger.ErrorContext, F.ToString("Timer \"", label, "\" doesn't exist."))
		return goja.Undefined()
	}
	writeLog(c.class.Runtime(), logger.ContextLogger.InfoContext, F.ToString(label, ": ", time.Since(startTime)))
	return goja.Undefined()
}

func (c *Console) trace(call goja.FunctionCall) any {
	return c.log(logger.ContextLogger.TraceContext, call.Arguments)
}

func (c *Console) warn(call goja.FunctionCall) any {
	return c.log(logger.ContextLogger.WarnContext, call.Arguments)
}

func writeLog(runtime *goja.Runtime, logFunc func(logger.ContextLogger, context.Context, ...any), message string) {
	var (
		ctx     context.Context
		sLogger logger.ContextLogger
	)
	boxCtx := boxctx.FromRuntime(runtime)
	if boxCtx != nil {
		ctx = boxCtx.Context
		sLogger = boxCtx.Logger
	} else {
		ctx = context.Background()
		sLogger = sLog.StdLogger()
	}
	logFunc(sLogger, ctx, message)
}

func format(runtime *goja.Runtime, b *bytes.Buffer, f string, args ...goja.Value) {
	pct := false
	argNum := 0
	for _, chr := range f {
		if pct {
			if argNum < len(args) {
				if format1(runtime, chr, args[argNum], b) {
					argNum++
				}
			} else {
				b.WriteByte('%')
				b.WriteRune(chr)
			}
			pct = false
		} else {
			if chr == '%' {
				pct = true
			} else {
				b.WriteRune(chr)
			}
		}
	}

	for _, arg := range args[argNum:] {
		b.WriteByte(' ')
		b.WriteString(arg.String())
	}
}

func format1(runtime *goja.Runtime, f rune, val goja.Value, w *bytes.Buffer) bool {
	switch f {
	case 's':
		w.WriteString(val.String())
	case 'd':
		w.WriteString(val.ToNumber().String())
	case 'j':
		if json, ok := runtime.Get("JSON").(*goja.Object); ok {
			if stringify, ok := goja.AssertFunction(json.Get("stringify")); ok {
				res, err := stringify(json, val)
				if err != nil {
					panic(err)
				}
				w.WriteString(res.String())
			}
		}
	case '%':
		w.WriteByte('%')
		return false
	default:
		w.WriteByte('%')
		w.WriteRune(f)
		return false
	}
	return true
}
