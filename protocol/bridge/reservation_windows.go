//go:build windows && (amd64 || 386)

package bridge

import (
	"encoding/binary"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

// SIO_ACQUIRE_PORT_RESERVATION = _WSAIOW(IOC_VENDOR, 100):
// IOC_IN | IOC_VENDOR | 100. Despite the write-only direction code, the
// reservation result is written to the WSAIoctl output buffer; using
// _WSAIORW instead is rejected with WSAEOPNOTSUPP.
const sioAcquirePortReservation uint32 = 0x80000000 | 0x18000000 | 100

// portReservation holds a runtime port block acquired from the host TCP/IP
// stack. Runtime reservation records are protocol- and family-agnostic:
// one reservation excludes the block from ephemeral auto-assignment for
// TCP and UDP sockets of both address families (and a specific reservation
// request for numbers covered by any existing record fails with
// WSAEADDRINUSE, whatever its protocol). Explicit binds inside the block
// are rejected for the reserving protocol but still allowed for others.
// Closing the socket releases the reservation.
type portReservation struct {
	socket    windows.Handle
	startPort uint16
}

func acquirePortReservation(family, socketType, protocol int, count uint16) (*portReservation, error) {
	socket, err := windows.Socket(family, socketType, protocol)
	if err != nil {
		return nil, E.Cause(err, "create reservation socket")
	}
	// INET_PORT_RANGE { USHORT StartPort; USHORT NumberOfPorts; }.
	// StartPort 0 requests a runtime (wildcard) reservation.
	var in [4]byte
	binary.LittleEndian.PutUint16(in[0:2], 0)
	binary.LittleEndian.PutUint16(in[2:4], count)
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
		return nil, E.Cause(err, "acquire port reservation")
	}
	// StartPort is returned in network byte order (as documented for
	// INET_PORT_RANGE); NumberOfPorts is a plain host-order count.
	startPort := binary.BigEndian.Uint16(out[0:2])
	reservedCount := binary.LittleEndian.Uint16(out[2:4])
	if startPort == 0 || reservedCount < count {
		windows.Closesocket(socket)
		return nil, E.New("acquire port reservation: stack returned ", reservedCount, " of ", count, " ports")
	}
	return &portReservation{
		socket:    socket,
		startPort: startPort,
	}, nil
}

func (r *portReservation) Close() {
	if r == nil {
		return
	}
	windows.Closesocket(r.socket)
}
