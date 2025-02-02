package sgstore

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing/service"

	"github.com/dop251/goja"
)

type SurgePersistentStore struct {
	vm        *goja.Runtime
	cacheFile adapter.CacheFile
	data      map[string]string
	tag       string
}

func Enable(vm *goja.Runtime, ctx context.Context) {
	object := vm.NewObject()
	cacheFile := service.FromContext[adapter.CacheFile](ctx)
	tag := vm.Get("$script").(*goja.Object).Get("name").String()
	store := &SurgePersistentStore{
		vm:        vm,
		cacheFile: cacheFile,
		tag:       tag,
	}
	if cacheFile == nil {
		store.data = make(map[string]string)
	}
	object.Set("read", store.js_read)
	object.Set("write", store.js_write)
	vm.Set("$persistentStore", object)
}

func (s *SurgePersistentStore) js_read(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) > 1 {
		panic(s.vm.NewTypeError("invalid arguments"))
	}
	key := jsc.AssertString(s.vm, call.Argument(0), "key", true)
	if key == "" {
		key = s.tag
	}
	var value string
	if s.cacheFile != nil {
		value = s.cacheFile.SurgePersistentStoreRead(key)
	} else {
		value = s.data[key]
	}
	if value == "" {
		return goja.Null()
	} else {
		return s.vm.ToValue(value)
	}
}

func (s *SurgePersistentStore) js_write(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 || len(call.Arguments) > 2 {
		panic(s.vm.NewTypeError("invalid arguments"))
	}
	data := jsc.AssertString(s.vm, call.Argument(0), "data", true)
	key := jsc.AssertString(s.vm, call.Argument(1), "key", true)
	if key == "" {
		key = s.tag
	}
	if s.cacheFile != nil {
		err := s.cacheFile.SurgePersistentStoreWrite(key, data)
		if err != nil {
			panic(s.vm.NewGoError(err))
		}
	} else {
		s.data[key] = data
	}
	return goja.Undefined()
}
