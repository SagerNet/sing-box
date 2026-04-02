package libbox

import "time"

func TriggerGoPanic() {
	time.AfterFunc(200*time.Millisecond, func() {
		panic("debug go crash")
	})
}
