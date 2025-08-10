package process

import (
	"context"
	"encoding/binary"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	N "github.com/sagernet/sing/common/network"

	"golang.org/x/sys/unix"
)

var _ Searcher = (*darwinSearcher)(nil)

type darwinSearcher struct{}

func NewSearcher(_ Config) (Searcher, error) {
	return &darwinSearcher{}, nil
}

func (d *darwinSearcher) FindProcessInfo(ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*Info, error) {
	processName, err := findProcessName(network, source.Addr(), int(source.Port()))
	if err != nil {
		return nil, err
	}
	return &Info{ProcessPath: processName, UserId: -1}, nil
}

var structSize = func() int {
	value, _ := syscall.Sysctl("kern.osrelease")
	major, _, _ := strings.Cut(value, ".")
	n, _ := strconv.ParseInt(major, 10, 64)
	switch true {
	case n >= 22:
		return 408
	default:
		// from darwin-xnu/bsd/netinet/in_pcblist.c:get_pcblist_n
		// size/offset are round up (aligned) to 8 bytes in darwin
		// rup8(sizeof(xinpcb_n)) + rup8(sizeof(xsocket_n)) +
		// 2 * rup8(sizeof(xsockbuf_n)) + rup8(sizeof(xsockstat_n))
		return 384
	}
}()

func findProcessName(network string, ip netip.Addr, port int) (string, error) {
	var spath string
	switch network {
	case N.NetworkTCP:
		spath = "net.inet.tcp.pcblist_n"
	case N.NetworkUDP:
		spath = "net.inet.udp.pcblist_n"
	default:
		return "", os.ErrInvalid
	}

	isIPv4 := ip.Is4()

	value, err := unix.SysctlRaw(spath)
	if err != nil {
		return "", err
	}

	buf := value

	// from darwin-xnu/bsd/netinet/in_pcblist.c:get_pcblist_n
	// size/offset are round up (aligned) to 8 bytes in darwin
	// rup8(sizeof(xinpcb_n)) + rup8(sizeof(xsocket_n)) +
	// 2 * rup8(sizeof(xsockbuf_n)) + rup8(sizeof(xsockstat_n))
	itemSize := structSize
	if network == N.NetworkTCP {
		// rup8(sizeof(xtcpcb_n))
		itemSize += 208
	}

	var fallbackUDPProcess string
	// skip the first xinpgen(24 bytes) block
	for i := 24; i+itemSize <= len(buf); i += itemSize {
		// offset of xinpcb_n and xsocket_n
		inp, so := i, i+104

		srcPort := binary.BigEndian.Uint16(buf[inp+18 : inp+20])
		if uint16(port) != srcPort {
			continue
		}

		// xinpcb_n.inp_vflag
		flag := buf[inp+44]

		var srcIP netip.Addr
		srcIsIPv4 := false
		switch {
		case flag&0x1 > 0 && isIPv4:
			// ipv4
			srcIP = netip.AddrFrom4([4]byte(buf[inp+76 : inp+80]))
			srcIsIPv4 = true
		case flag&0x2 > 0 && !isIPv4:
			// ipv6
			srcIP = netip.AddrFrom16([16]byte(buf[inp+64 : inp+80]))
		default:
			continue
		}

		if ip == srcIP {
			// xsocket_n.so_last_pid
			pid := readNativeUint32(buf[so+68 : so+72])
			return getExecPathFromPID(pid)
		}

		// udp packet connection may be not equal with srcIP
		if network == N.NetworkUDP && srcIP.IsUnspecified() && isIPv4 == srcIsIPv4 {
			pid := readNativeUint32(buf[so+68 : so+72])
			fallbackUDPProcess, _ = getExecPathFromPID(pid)
		}
	}

	if network == N.NetworkUDP && len(fallbackUDPProcess) > 0 {
		return fallbackUDPProcess, nil
	}

	return "", ErrNotFound
}

func getExecPathFromPID(pid uint32) (string, error) {
	const (
		procpidpathinfo     = 0xb
		procpidpathinfosize = 1024
		proccallnumpidinfo  = 0x2
	)
	buf := make([]byte, procpidpathinfosize)
	_, _, errno := syscall.Syscall6(
		syscall.SYS_PROC_INFO,
		proccallnumpidinfo,
		uintptr(pid),
		procpidpathinfo,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		procpidpathinfosize)
	if errno != 0 {
		return "", errno
	}

	return unix.ByteSliceToString(buf), nil
}

func readNativeUint32(b []byte) uint32 {
	return *(*uint32)(unsafe.Pointer(&b[0]))
}
