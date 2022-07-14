//go:build !windows

package wininet

import "os"

func ClearSystemProxy() error {
	return os.ErrInvalid
}

func SetSystemProxy(proxy string, bypass string) error {
	return os.ErrInvalid
}
