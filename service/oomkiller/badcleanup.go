//go:build badlinkname && !goexperiment.jsonv2

package oomkiller

import (
	"sync"
	_ "unsafe"
)

//go:linkname jsonEncoderCache encoding/json.encoderCache
var jsonEncoderCache sync.Map

//go:linkname jsonFieldCache encoding/json.fieldCache
var jsonFieldCache sync.Map

//go:linkname contextJSONEncoderCache github.com/sagernet/sing/common/json/internal/contextjson.encoderCache
var contextJSONEncoderCache sync.Map

//go:linkname contextJSONFieldCache github.com/sagernet/sing/common/json/internal/contextjson.fieldCache
var contextJSONFieldCache sync.Map

func badCleanup() {
	jsonEncoderCache.Clear()
	jsonFieldCache.Clear()
	contextJSONEncoderCache.Clear()
	contextJSONFieldCache.Clear()
}
