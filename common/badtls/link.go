//go:build go1.20 && !go.1.21

package badtls

import (
	"reflect"
	_ "unsafe"
)

const (
	maxPlaintext    = 16384 // maximum plaintext payload length
	recordHeaderLen = 5     // record header length
)

//go:linkname errShutdown crypto/tls.errShutdown
var errShutdown error

//go:linkname incSeq crypto/tls.(*halfConn).incSeq
func incSeq(conn uintptr)

//go:linkname valueInterface reflect.valueInterface
func valueInterface(v reflect.Value, safe bool) any
