package warning

import (
	"sync"

	"github.com/sagernet/sing-box/log"
)

type Warning struct {
	logger    log.Logger
	check     CheckFunc
	message   string
	checkOnce sync.Once
}

type CheckFunc = func() bool

func New(checkFunc CheckFunc, message string) Warning {
	return Warning{
		check:   checkFunc,
		message: message,
	}
}

func (w *Warning) Check() {
	w.checkOnce.Do(func() {
		if w.check() {
			log.Warn(w.message)
		}
	})
}
