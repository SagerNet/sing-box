package tlsspoof

import (
	"encoding/binary"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"syscall"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

const PlatformSupported = true

// Offsets into xinpcb_n within each net.inet.tcp.pcblist_n record, identical
// to the values used by common/process/searcher_darwin_shared.go.
const (
	darwinXinpgenSize       = 24
	darwinXsocketOffset     = 104
	darwinXinpcbForeignPort = 16
	darwinXinpcbLocalPort   = 18
	darwinXinpcbVFlag       = 44
	darwinXinpcbForeignAddr = 48
	darwinXinpcbLocalAddr   = 64
	darwinXinpcbIPv4Offset  = 12

	darwinTCPExtraSize = 208

	darwinXtcpcbSndNxtOffset = 56
	darwinXtcpcbRcvNxtOffset = 80
)

var darwinStructSize = sync.OnceValue(func() int {
	value, _ := syscall.Sysctl("kern.osrelease")
	major, _, _ := strings.Cut(value, ".")
	n, _ := strconv.ParseInt(major, 10, 64)
	if n >= 22 {
		return 408
	}
	return 384
})

type darwinSpoofer struct {
	method      Method
	src         netip.AddrPort
	dst         netip.AddrPort
	rawFD       int
	rawSockAddr unix.Sockaddr
	sendNext    uint32
	receiveNext uint32
}

func newRawSpoofer(conn net.Conn, method Method) (Spoofer, error) {
	_, src, dst, err := tcpEndpoints(conn)
	if err != nil {
		return nil, err
	}
	fd, sockaddr, err := openDarwinRawSocket(dst)
	if err != nil {
		return nil, err
	}
	sendNext, receiveNext, err := readDarwinTCPSequence(src, dst)
	if err != nil {
		unix.Close(fd)
		return nil, err
	}
	return &darwinSpoofer{
		method:      method,
		src:         src,
		dst:         dst,
		rawFD:       fd,
		rawSockAddr: sockaddr,
		sendNext:    sendNext,
		receiveNext: receiveNext,
	}, nil
}

// readDarwinTCPSequence scans net.inet.tcp.pcblist_n for the PCB that matches
// src -> dst and returns (snd_nxt, rcv_nxt). These live in xtcpcb_n at the end
// of each record; see darwin-xnu bsd/netinet/in_pcblist.c:get_pcblist_n.
func readDarwinTCPSequence(src, dst netip.AddrPort) (uint32, uint32, error) {
	buffer, err := unix.SysctlRaw("net.inet.tcp.pcblist_n")
	if err != nil {
		return 0, 0, E.Cause(err, "sysctl net.inet.tcp.pcblist_n")
	}
	structSize := darwinStructSize()
	itemSize := structSize + darwinTCPExtraSize
	for i := darwinXinpgenSize; i+itemSize <= len(buffer); i += itemSize {
		inpcb := buffer[i : i+darwinXsocketOffset]
		xtcpcb := buffer[i+structSize : i+itemSize]
		localPort := binary.BigEndian.Uint16(inpcb[darwinXinpcbLocalPort : darwinXinpcbLocalPort+2])
		remotePort := binary.BigEndian.Uint16(inpcb[darwinXinpcbForeignPort : darwinXinpcbForeignPort+2])
		if localPort != src.Port() || remotePort != dst.Port() {
			continue
		}
		versionFlag := inpcb[darwinXinpcbVFlag]
		var localAddr, remoteAddr netip.Addr
		switch {
		case versionFlag&0x1 != 0:
			localAddr = netip.AddrFrom4([4]byte(inpcb[darwinXinpcbLocalAddr+darwinXinpcbIPv4Offset : darwinXinpcbLocalAddr+darwinXinpcbIPv4Offset+4]))
			remoteAddr = netip.AddrFrom4([4]byte(inpcb[darwinXinpcbForeignAddr+darwinXinpcbIPv4Offset : darwinXinpcbForeignAddr+darwinXinpcbIPv4Offset+4]))
		case versionFlag&0x2 != 0:
			localAddr = netip.AddrFrom16([16]byte(inpcb[darwinXinpcbLocalAddr : darwinXinpcbLocalAddr+16]))
			remoteAddr = netip.AddrFrom16([16]byte(inpcb[darwinXinpcbForeignAddr : darwinXinpcbForeignAddr+16]))
		default:
			continue
		}
		if localAddr.Unmap() != src.Addr() || remoteAddr.Unmap() != dst.Addr() {
			continue
		}
		sendNext := binary.NativeEndian.Uint32(xtcpcb[darwinXtcpcbSndNxtOffset : darwinXtcpcbSndNxtOffset+4])
		receiveNext := binary.NativeEndian.Uint32(xtcpcb[darwinXtcpcbRcvNxtOffset : darwinXtcpcbRcvNxtOffset+4])
		return sendNext, receiveNext, nil
	}
	return 0, 0, E.New("tls_spoof: connection ", src, "->", dst, " not found in pcblist_n")
}

func openDarwinRawSocket(dst netip.AddrPort) (int, unix.Sockaddr, error) {
	if !dst.Addr().Is4() {
		// macOS does not expose IPV6_HDRINCL; raw AF_INET6 injection would
		// require either BPF link-layer writes or kernel-side IPv6 header
		// synthesis, neither of which is implemented here.
		return -1, nil, E.New("tls_spoof: IPv6 not supported on darwin")
	}
	return openIPv4RawSocket(dst)
}

func (s *darwinSpoofer) Inject(payload []byte) error {
	frame, err := buildSpoofFrame(s.method, s.src, s.dst, s.sendNext, s.receiveNext, payload)
	if err != nil {
		return err
	}
	// Darwin inherits the historical BSD quirk: with IP_HDRINCL the kernel
	// expects ip_len and ip_off in host byte order, not network byte order.
	// Apple's rip_output swaps them back before transmission. This does not
	// apply to IPv6.
	if s.src.Addr().Is4() {
		totalLen := binary.BigEndian.Uint16(frame[2:4])
		binary.NativeEndian.PutUint16(frame[2:4], totalLen)
		fragOff := binary.BigEndian.Uint16(frame[6:8])
		binary.NativeEndian.PutUint16(frame[6:8], fragOff)
	}
	err = unix.Sendto(s.rawFD, frame, 0, s.rawSockAddr)
	if err != nil {
		return E.Cause(err, "sendto raw socket")
	}
	return nil
}

func (s *darwinSpoofer) Close() error {
	if s.rawFD < 0 {
		return nil
	}
	err := unix.Close(s.rawFD)
	s.rawFD = -1
	return err
}
