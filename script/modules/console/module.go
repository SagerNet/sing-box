package console

import (
	"bytes"
	"context"

	"github.com/sagernet/sing-box/script/modules/require"
	"github.com/sagernet/sing/common/logger"

	"github.com/dop251/goja"
)

const ModuleName = "console"

type Console struct {
	vm *goja.Runtime
}

func (c *Console) log(ctx context.Context, p func(ctx context.Context, values ...any)) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		var buffer bytes.Buffer
		var format string
		if arg := call.Argument(0); !goja.IsUndefined(arg) {
			format = arg.String()
		}
		var args []goja.Value
		if len(call.Arguments) > 0 {
			args = call.Arguments[1:]
		}
		c.Format(&buffer, format, args...)
		p(ctx, buffer.String())
		return nil
	}
}

func (c *Console) Format(b *bytes.Buffer, f string, args ...goja.Value) {
	pct := false
	argNum := 0
	for _, chr := range f {
		if pct {
			if argNum < len(args) {
				if c.format(chr, args[argNum], b) {
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

func (c *Console) format(f rune, val goja.Value, w *bytes.Buffer) bool {
	switch f {
	case 's':
		w.WriteString(val.String())
	case 'd':
		w.WriteString(val.ToNumber().String())
	case 'j':
		if json, ok := c.vm.Get("JSON").(*goja.Object); ok {
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

func Require(ctx context.Context, logger logger.ContextLogger) require.ModuleLoader {
	return func(runtime *goja.Runtime, module *goja.Object) {
		c := &Console{
			vm: runtime,
		}
		o := module.Get("exports").(*goja.Object)
		o.Set("log", c.log(ctx, logger.DebugContext))
		o.Set("error", c.log(ctx, logger.ErrorContext))
		o.Set("warn", c.log(ctx, logger.WarnContext))
		o.Set("info", c.log(ctx, logger.InfoContext))
		o.Set("debug", c.log(ctx, logger.DebugContext))
	}
}

func Enable(runtime *goja.Runtime) {
	runtime.Set("console", require.Require(runtime, ModuleName))
}
