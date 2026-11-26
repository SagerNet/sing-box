//go:build badlinkname && goexperiment.jsonv2

package oomkiller

import (
	"sync"
	_ "unsafe"
)

//go:linkname jsonArshalerCache encoding/json/v2.lookupArshalerCache
var jsonArshalerCache sync.Map

//go:linkname contextJSONEncoderCache github.com/sagernet/sing/common/json/internal/contextjson.encoderCache
var contextJSONEncoderCache sync.Map

//go:linkname contextJSONFieldCache github.com/sagernet/sing/common/json/internal/contextjson.fieldCache
var contextJSONFieldCache sync.Map

func badCleanup() {
	jsonArshalerCache.Clear()
	contextJSONEncoderCache.Clear()
	contextJSONFieldCache.Clear()
}
