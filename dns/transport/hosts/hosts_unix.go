//go:build !windows

package hosts

func defaultPath() (string, error) {
	return "/etc/hosts", nil
}
