//go:build linux || darwin

package libbox

import (
	C "github.com/sagernet/sing-box/constant"

	"github.com/dustin/go-humanize"
)

func SetBasePath(path string) {
	C.SetBasePath(path)
}

func Version() string {
	return C.Version
}

func FormatBytes(length int64) string {
	return humanize.IBytes(uint64(length))
}
