package libbox

import (
	"time"
	"unsafe"
)

func TriggerGoPanic() {
	time.AfterFunc(200*time.Millisecond, func() {
		*(*int)(unsafe.Pointer(uintptr(0))) = 0
	})
}
