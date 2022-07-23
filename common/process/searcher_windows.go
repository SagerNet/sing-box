package process

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"syscall"
	"unsafe"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

var _ Searcher = (*windowsSearcher)(nil)

type windowsSearcher struct{}

func NewSearcher(logger log.ContextLogger) (Searcher, error) {
	err := initWin32API()
	if err != nil {
		return nil, E.Cause(err, "init win32 api")
	}
	return &windowsSearcher{}, nil
}

var (
	modiphlpapi                    = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable        = modiphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtendedUdpTable        = modiphlpapi.NewProc("GetExtendedUdpTable")
	modkernel32                    = windows.NewLazySystemDLL("kernel32.dll")
	procQueryFullProcessImageNameW = modkernel32.NewProc("QueryFullProcessImageNameW")
)

func initWin32API() error {
	err := modiphlpapi.Load()
	if err != nil {
		return E.Cause(err, "load iphlpapi.dll")
	}

	err = procGetExtendedTcpTable.Find()
	if err != nil {
		return E.Cause(err, "load iphlpapi::GetExtendedTcpTable")
	}

	err = procGetExtendedUdpTable.Find()
	if err != nil {
		return E.Cause(err, "load iphlpapi::GetExtendedUdpTable")
	}

	err = modkernel32.Load()
	if err != nil {
		return E.Cause(err, "load kernel32.dll")
	}

	err = procQueryFullProcessImageNameW.Find()
	if err != nil {
		return E.Cause(err, "load kernel32::QueryFullProcessImageNameW")
	}

	return nil
}

func (s *windowsSearcher) FindProcessInfo(ctx context.Context, network string, srcIP netip.Addr, srcPort int) (*Info, error) {
	processName, err := findProcessName(network, srcIP, srcPort)
	if err != nil {
		return nil, err
	}
	return &Info{ProcessPath: processName, UserId: -1}, nil
}

func findProcessName(network string, ip netip.Addr, srcPort int) (string, error) {
	family := windows.AF_INET
	if ip.Is6() {
		family = windows.AF_INET6
	}

	const (
		tcpTablePidConn = 4
		udpTablePid     = 1
	)

	var class int
	var fn uintptr
	switch network {
	case C.NetworkTCP:
		fn = procGetExtendedTcpTable.Addr()
		class = tcpTablePidConn
	case C.NetworkUDP:
		fn = procGetExtendedUdpTable.Addr()
		class = udpTablePid
	default:
		return "", os.ErrInvalid
	}

	buf, err := getTransportTable(fn, family, class)
	if err != nil {
		return "", err
	}

	s := newSearcher(family == windows.AF_INET, network == C.NetworkTCP)

	pid, err := s.Search(buf, ip, uint16(srcPort))
	if err != nil {
		return "", err
	}
	return getExecPathFromPID(pid)
}

type searcher struct {
	itemSize int
	port     int
	ip       int
	ipSize   int
	pid      int
	tcpState int
}

func (s *searcher) Search(b []byte, ip netip.Addr, port uint16) (uint32, error) {
	n := int(readNativeUint32(b[:4]))
	itemSize := s.itemSize
	for i := 0; i < n; i++ {
		row := b[4+itemSize*i : 4+itemSize*(i+1)]

		if s.tcpState >= 0 {
			tcpState := readNativeUint32(row[s.tcpState : s.tcpState+4])
			// MIB_TCP_STATE_ESTAB, only check established connections for TCP
			if tcpState != 5 {
				continue
			}
		}

		// according to MSDN, only the lower 16 bits of dwLocalPort are used and the port number is in network endian.
		// this field can be illustrated as follows depends on different machine endianess:
		//     little endian: [ MSB LSB  0   0  ]   interpret as native uint32 is ((LSB<<8)|MSB)
		//       big  endian: [  0   0  MSB LSB ]   interpret as native uint32 is ((MSB<<8)|LSB)
		// so we need an syscall.Ntohs on the lower 16 bits after read the port as native uint32
		srcPort := syscall.Ntohs(uint16(readNativeUint32(row[s.port : s.port+4])))
		if srcPort != port {
			continue
		}

		srcIP, _ := netip.AddrFromSlice(row[s.ip : s.ip+s.ipSize])
		// windows binds an unbound udp socket to 0.0.0.0/[::] while first sendto
		if ip != srcIP && (!srcIP.IsUnspecified() || s.tcpState != -1) {
			continue
		}

		pid := readNativeUint32(row[s.pid : s.pid+4])
		return pid, nil
	}
	return 0, ErrNotFound
}

func newSearcher(isV4, isTCP bool) *searcher {
	var itemSize, port, ip, ipSize, pid int
	tcpState := -1
	switch {
	case isV4 && isTCP:
		// struct MIB_TCPROW_OWNER_PID
		itemSize, port, ip, ipSize, pid, tcpState = 24, 8, 4, 4, 20, 0
	case isV4 && !isTCP:
		// struct MIB_UDPROW_OWNER_PID
		itemSize, port, ip, ipSize, pid = 12, 4, 0, 4, 8
	case !isV4 && isTCP:
		// struct MIB_TCP6ROW_OWNER_PID
		itemSize, port, ip, ipSize, pid, tcpState = 56, 20, 0, 16, 52, 48
	case !isV4 && !isTCP:
		// struct MIB_UDP6ROW_OWNER_PID
		itemSize, port, ip, ipSize, pid = 28, 20, 0, 16, 24
	}

	return &searcher{
		itemSize: itemSize,
		port:     port,
		ip:       ip,
		ipSize:   ipSize,
		pid:      pid,
		tcpState: tcpState,
	}
}

func getTransportTable(fn uintptr, family int, class int) ([]byte, error) {
	for size, buf := uint32(8), make([]byte, 8); ; {
		ptr := unsafe.Pointer(&buf[0])
		err, _, _ := syscall.SyscallN(fn, uintptr(ptr), uintptr(unsafe.Pointer(&size)), 0, uintptr(family), uintptr(class), 0)

		switch err {
		case 0:
			return buf, nil
		case uintptr(syscall.ERROR_INSUFFICIENT_BUFFER):
			buf = make([]byte, size)
		default:
			return nil, fmt.Errorf("syscall error: %d", err)
		}
	}
}

func readNativeUint32(b []byte) uint32 {
	return *(*uint32)(unsafe.Pointer(&b[0]))
}

func getExecPathFromPID(pid uint32) (string, error) {
	// kernel process starts with a colon in order to distinguish with normal processes
	switch pid {
	case 0:
		// reserved pid for system idle process
		return ":System Idle Process", nil
	case 4:
		// reserved pid for windows kernel image
		return ":System", nil
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, syscall.MAX_LONG_PATH)
	size := uint32(len(buf))
	r1, _, err := syscall.SyscallN(
		procQueryFullProcessImageNameW.Addr(),
		uintptr(h),
		uintptr(1),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r1 == 0 {
		return "", err
	}
	return syscall.UTF16ToString(buf[:size]), nil
}
