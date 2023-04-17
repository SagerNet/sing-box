package libbox

import "golang.org/x/sys/unix"

func getTunnelName(fd int32) (string, error) {
	return unix.GetsockoptString(
		int(fd),
		2, /* #define SYSPROTO_CONTROL 2 */
		2, /* #define UTUN_OPT_IFNAME 2 */
	)
}
