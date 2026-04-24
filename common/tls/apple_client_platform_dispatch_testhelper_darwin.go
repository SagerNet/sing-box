//go:build darwin && cgo

package tls

/*
#include <stdlib.h>
#include "apple_client_platform_darwin.h"
*/
import "C"

import "unsafe"

func appleTLSCopyDispatchDataForTest(first, second []byte, buffer []byte) (int, string) {
	var firstPtr unsafe.Pointer
	if len(first) > 0 {
		firstPtr = C.CBytes(first)
		defer C.free(firstPtr)
	}
	var secondPtr unsafe.Pointer
	if len(second) > 0 {
		secondPtr = C.CBytes(second)
		defer C.free(secondPtr)
	}
	var bufferPtr unsafe.Pointer
	if len(buffer) > 0 {
		bufferPtr = unsafe.Pointer(&buffer[0])
	}
	var errPtr *C.char
	n := C.box_apple_tls_copy_dispatch_data_for_test(
		firstPtr,
		C.size_t(len(first)),
		secondPtr,
		C.size_t(len(second)),
		bufferPtr,
		C.size_t(len(buffer)),
		&errPtr,
	)
	if errPtr == nil {
		return int(n), ""
	}
	errorMessage := C.GoString(errPtr)
	C.free(unsafe.Pointer(errPtr))
	return int(n), errorMessage
}
