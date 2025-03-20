package jsc

import (
	"time"
	_ "unsafe"

	"github.com/dop251/goja"
)

func TimeToValue(runtime *goja.Runtime, time time.Time) goja.Value {
	return runtimeNewDateObject(runtime, time, true, runtimeGetDatePrototype(runtime))
}

//go:linkname runtimeNewDateObject github.com/dop251/goja.(*Runtime).newDateObject
func runtimeNewDateObject(r *goja.Runtime, t time.Time, isSet bool, proto *goja.Object) *goja.Object

//go:linkname runtimeGetDatePrototype github.com/dop251/goja.(*Runtime).getDatePrototype
func runtimeGetDatePrototype(r *goja.Runtime) *goja.Object
