//go:build !(windows || linux || darwin)

package settings

import (
	"os"
	"time"
)

func SetSystemTime(nowTime time.Time) error {
	return os.ErrInvalid
}
