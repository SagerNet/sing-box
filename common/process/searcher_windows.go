package process

import (
	"context"
	"errors"
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
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
	var info *Info
	if N.NetworkName(network) == N.NetworkTCP {
		pid, err := winiphlpapi.FindTCPPid(source, destination)
		if err != nil {
			return nil, err
		}
		info = &Info{ProcessID: pid}
	} else {
		pid, err := winiphlpapi.FindUDPPid(source)
		if err != nil {
			return nil, err
		}
		info = &Info{ProcessID: pid}
	}
	if info == nil {
		return nil, ErrNotFound
	}
	var err error
	info.ProcessPath, err = getProcessPath(info.ProcessID)
	return info, err
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
	var size uint32
	err = windows.QueryFullProcessImageName(handle, 0, nil, &size)
	if !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
		return "", err
	}
	buf := make([]uint16, size)
	err = windows.QueryFullProcessImageName(handle, 0, &buf[0], &size)
	if err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf), nil
}
