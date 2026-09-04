package process

import (
	"context"
	"net/netip"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/winiphlpapi"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
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

func (s *windowsSearcher) ResetCache() {
}

func (s *windowsSearcher) Close() error {
	return nil
}

func (s *windowsSearcher) FindProcessInfo(ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*adapter.ConnectionOwner, error) {
	owner, err := winiphlpapi.FindSocketOwner(network, source)
	if err != nil {
		return nil, err
	}
	path, err := getProcessPath(owner.Pid)
	if err != nil {
		return &adapter.ConnectionOwner{ProcessID: owner.Pid, UserId: -1}, err
	}
	processPaths := []string{path}
	if owner.ServiceName != "" {
		servicePath, serviceErr := getServiceDLLPath(owner.ServiceName)
		if serviceErr == nil {
			processPaths = []string{servicePath, path}
		}
	}
	return &adapter.ConnectionOwner{ProcessID: owner.Pid, ProcessPaths: processPaths, UserId: -1}, nil
}

func getServiceDLLPath(serviceName string) (string, error) {
	serviceKeyPath := `SYSTEM\CurrentControlSet\Services\` + serviceName
	var lastErr error
	for _, keyPath := range []string{serviceKeyPath + `\Parameters`, serviceKeyPath} {
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
		if err != nil {
			lastErr = err
			continue
		}
		value, valueType, err := key.GetStringValue("ServiceDll")
		key.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if valueType == registry.EXPAND_SZ {
			return registry.ExpandString(value)
		}
		return value, nil
	}
	return "", lastErr
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
