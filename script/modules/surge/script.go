package surge

import (
	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing-box/script/modules/boxctx"
	F "github.com/sagernet/sing/common/format"
)

type Script struct {
	class      jsc.Class[*Module, *Script]
	ScriptType string
}

func createScript(module *Module) jsc.Class[*Module, *Script] {
	class := jsc.NewClass[*Module, *Script](module)
	class.DefineField("name", (*Script).getName, nil)
	class.DefineField("type", (*Script).getType, nil)
	class.DefineField("startTime", (*Script).getStartTime, nil)
	return class
}

func (s *Script) getName() any {
	return F.ToString("script:", boxctx.MustFromRuntime(s.class.Runtime()).Tag)
}

func (s *Script) getType() any {
	return s.ScriptType
}

func (s *Script) getStartTime() any {
	return boxctx.MustFromRuntime(s.class.Runtime()).StartedAt
}
