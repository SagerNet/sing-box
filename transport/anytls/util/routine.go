package util

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/sagernet/sing-box/log"
)

func StartRoutine(ctx context.Context, d time.Duration, f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("[BUG]", r, string(debug.Stack()))
			}
		}()
		for {
			time.Sleep(d)
			f()
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()
}
