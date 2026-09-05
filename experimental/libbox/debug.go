package libbox

import (
	"runtime"
	"time"
	"unsafe"
)

func TriggerGoPanic() {
	time.AfterFunc(200*time.Millisecond, func() {
		*(*int)(unsafe.Pointer(uintptr(0))) = 0
	})
}

func TriggerGoHang(seconds int32) {
	time.Sleep(time.Duration(seconds) * time.Second)
}

func GoroutineDump() string {
	buffer := make([]byte, 64*1024)
	for {
		n := runtime.Stack(buffer, true)
		if n < len(buffer) {
			return string(buffer[:n])
		}
		buffer = make([]byte, 2*len(buffer))
	}
}
