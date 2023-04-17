//go:build !(darwin || linux)

package libbox

import "os"

func getTunnelName(fd int32) (string, error) {
	return "", os.ErrInvalid
}
