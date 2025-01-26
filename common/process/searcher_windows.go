package process

import (
	"context"
	"net/netip"
	"syscall"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/winiphlpapi"

	"golang.org/x/sys/windows"
)

var _ Searcher = (*windowsSearcher)(nil)

type windowsSearcher struct{}

func NewSearcher(_ Config) (Searcher, error) {
	err := initWin32API()
	if err != nil {
		return nil, E.Cause(err, "init win32 api")
	}
	return &windowsSearcher{}, nil
}

func initWin32API() error {
	return winiphlpapi.LoadExtendedTable()
}

func (s *windowsSearcher) FindProcessInfo(ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*Info, error) {
	pid, err := winiphlpapi.FindPid(network, source)
	if err != nil {
		return nil, err
	}
	path, err := getProcessPath(pid)
	if err != nil {
		return &Info{ProcessID: pid, UserId: -1}, err
	}
	return &Info{ProcessID: pid, ProcessPath: path, UserId: -1}, nil
}

func getProcessPath(pid uint32) (string, error) {
	switch pid {
	case 0:
		return ":System Idle Process", nil
	case 4:
		return ":System", nil
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	size := uint32(syscall.MAX_LONG_PATH)
	buf := make([]uint16, syscall.MAX_LONG_PATH)
	err = windows.QueryFullProcessImageName(handle, 0, &buf[0], &size)
	if err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:size]), nil
}
