//go:build badlinkname

package oomkiller

import (
	"sync"
	_ "unsafe"
)

//go:linkname jsonFieldCache json.fieldCache
var jsonFieldCache sync.Map

//go:linkname contextJSONFieldCache github.com/sagernet/sing/common/json/internal/contextjson.fieldCache
var contextJSONFieldCache sync.Map

func badCleanup() {
	jsonFieldCache.Clear()
	contextJSONFieldCache.Clear()
}
