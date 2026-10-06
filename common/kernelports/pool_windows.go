package kernelports

import (
	"encoding/binary"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

const (
	blockSize uint16 = 1024
	maxBlocks int    = 8
)

// SIO_ACQUIRE_PORT_RESERVATION = _WSAIOW(IOC_VENDOR, 100):
// IOC_IN | IOC_VENDOR | 100. Despite the write-only direction code, the
// reservation result is written to the WSAIoctl output buffer; using
// _WSAIORW instead is rejected with WSAEOPNOTSUPP.
const sioAcquirePortReservation uint32 = 0x80000000 | 0x18000000 | 100

type Pool struct {
	tcp atomic.Pointer[[]tun.SelectorRange]
	udp atomic.Pointer[[]tun.SelectorRange]

	access sync.Mutex
	blocks map[uint8][]windows.Handle
}

func New() (*Pool, error) {
	pool := &Pool{blocks: make(map[uint8][]windows.Handle)}
	for _, protocol := range []uint8{uint8(header.TCPProtocolNumber), uint8(header.UDPProtocolNumber)} {
		_, err := pool.Acquire(protocol)
		if err != nil {
			pool.Close()
			return nil, err
		}
	}
	return pool, nil
}

// A runtime reservation only binds the protocol of the socket it was
// acquired on: both ephemeral auto-assignment and explicit binds of that
// protocol skip the block, while sockets of the other protocol still receive
// ports inside it (observed on Windows 11; the documentation does not say).
// Repeated wildcard requests from one process return distinct,
// non-contiguous blocks. Closing the socket releases the reservation.
func (p *Pool) Acquire(protocol uint8) (tun.SelectorRange, error) {
	socketType, socketProtocol := windows.SOCK_STREAM, windows.IPPROTO_TCP
	if protocol == uint8(header.UDPProtocolNumber) {
		socketType, socketProtocol = windows.SOCK_DGRAM, windows.IPPROTO_UDP
	} else {
		protocol = uint8(header.TCPProtocolNumber)
	}
	p.access.Lock()
	defer p.access.Unlock()
	if len(p.blocks[protocol]) >= maxBlocks {
		return tun.SelectorRange{}, ErrExhausted
	}
	socket, err := windows.Socket(windows.AF_INET, socketType, socketProtocol)
	if err != nil {
		return tun.SelectorRange{}, E.Cause(err, "create reservation socket")
	}
	// INET_PORT_RANGE { USHORT StartPort; USHORT NumberOfPorts; }.
	// StartPort 0 requests a runtime (wildcard) reservation.
	var in [4]byte
	binary.LittleEndian.PutUint16(in[0:2], 0)
	binary.LittleEndian.PutUint16(in[2:4], blockSize)
	// INET_PORT_RESERVATION_INSTANCE {
	//   INET_PORT_RESERVATION { USHORT StartPort; USHORT NumberOfPorts; };
	//   INET_PORT_RESERVATION_TOKEN { ULONG64 Token; };
	// } — the ULONG64 forces 8-byte alignment, so Token sits at offset 8.
	var out [16]byte
	var returned uint32
	err = windows.WSAIoctl(socket, sioAcquirePortReservation,
		&in[0], uint32(len(in)), &out[0], uint32(len(out)), &returned, nil, 0)
	if err != nil {
		windows.Closesocket(socket)
		return tun.SelectorRange{}, E.Cause(err, "acquire port reservation")
	}
	// StartPort is returned in network byte order (as documented for
	// INET_PORT_RANGE); NumberOfPorts is a plain host-order count.
	startPort := binary.BigEndian.Uint16(out[0:2])
	reservedCount := binary.LittleEndian.Uint16(out[2:4])
	if startPort == 0 || reservedCount < blockSize {
		windows.Closesocket(socket)
		return tun.SelectorRange{}, E.New("acquire port reservation: stack returned ", reservedCount, " of ", blockSize, " ports")
	}
	p.blocks[protocol] = append(p.blocks[protocol], socket)
	block := tun.SelectorRange{Start: startPort, Count: blockSize}
	ranges := p.ranges(protocol)
	var updated []tun.SelectorRange
	current := ranges.Load()
	if current != nil {
		updated = append(updated, *current...)
	}
	updated = append(updated, block)
	ranges.Store(&updated)
	return block, nil
}

func (p *Pool) ReserveSelector(protocol uint8, address netip.AddrPort) bool {
	return true
}

func (p *Pool) Close() error {
	p.access.Lock()
	defer p.access.Unlock()
	for _, sockets := range p.blocks {
		for _, socket := range sockets {
			windows.Closesocket(socket)
		}
	}
	clear(p.blocks)
	return nil
}
