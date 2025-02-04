package surge

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/script/jsc"
	"github.com/sagernet/sing-box/script/modules/boxctx"
	"github.com/sagernet/sing/service"

	"github.com/dop251/goja"
)

type PersistentStore struct {
	class         jsc.Class[*Module, *PersistentStore]
	cacheFile     adapter.CacheFile
	inMemoryCache *adapter.SurgeInMemoryCache
	tag           string
}

func createPersistentStore(module *Module) jsc.Class[*Module, *PersistentStore] {
	class := jsc.NewClass[*Module, *PersistentStore](module)
	class.DefineMethod("get", (*PersistentStore).get)
	class.DefineMethod("set", (*PersistentStore).set)
	class.DefineMethod("toString", (*PersistentStore).toString)
	return class
}

func newPersistentStore(class jsc.Class[*Module, *PersistentStore]) goja.Value {
	boxCtx := boxctx.MustFromRuntime(class.Runtime())
	return class.New(&PersistentStore{
		class:         class,
		cacheFile:     service.FromContext[adapter.CacheFile](boxCtx.Context),
		inMemoryCache: service.FromContext[adapter.ScriptManager](boxCtx.Context).SurgeCache(),
		tag:           boxCtx.Tag,
	})
}

func (s *PersistentStore) get(call goja.FunctionCall) any {
	key := jsc.AssertString(s.class.Runtime(), call.Argument(0), "key", true)
	if key == "" {
		key = s.tag
	}
	var value string
	if s.cacheFile != nil {
		value = s.cacheFile.SurgePersistentStoreRead(key)
	} else {
		s.inMemoryCache.RLock()
		value = s.inMemoryCache.Data[key]
		s.inMemoryCache.RUnlock()
	}
	if value == "" {
		return goja.Null()
	} else {
		return value
	}
}

func (s *PersistentStore) set(call goja.FunctionCall) any {
	data := jsc.AssertString(s.class.Runtime(), call.Argument(0), "data", true)
	key := jsc.AssertString(s.class.Runtime(), call.Argument(1), "key", true)
	if key == "" {
		key = s.tag
	}
	if s.cacheFile != nil {
		err := s.cacheFile.SurgePersistentStoreWrite(key, data)
		if err != nil {
			panic(s.class.Runtime().NewGoError(err))
		}
	} else {
		s.inMemoryCache.Lock()
		s.inMemoryCache.Data[key] = data
		s.inMemoryCache.Unlock()
	}
	return goja.Undefined()
}

func (s *PersistentStore) toString(call goja.FunctionCall) any {
	return "[sing-box Surge persistentStore]"
}
