//go:build windows

package usbip

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/common/usbipvhci"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

const (
	loopbackHost = "127.0.0.1"

	// The VHCI driver connects from kernel WSK and sends OP_REQ_IMPORT
	// immediately; a peer that stalls the handshake is not the driver.
	relayHandshakeTimeout = 5 * time.Second

	systemProcessID = 4
)

// windowsImportHost imports remote devices through the usbip-win2 UDE
// driver. The driver connects and speaks USB/IP itself, in-kernel, so it
// cannot accept sing-box's already-dialed (and possibly proxied) socket.
// Instead each Attach runs a one-shot loopback listener, points the
// driver at it, answers the driver's import handshake from the cached
// device info, and splices the loopback stream to the proxied server
// connection. All proxy/dialer behavior therefore stays in userspace.
type windowsImportHost struct {
	logger     log.ContextLogger
	controller *usbipvhci.Controller
}

func (h *windowsImportHost) Start() error {
	err := usbipvhci.EnsureDriver()
	if err != nil {
		return E.Cause(err, "install usbip-win2 VHCI driver")
	}
	controller, err := usbipvhci.Open()
	if err != nil {
		return err
	}
	h.controller = controller
	return nil
}

func (h *windowsImportHost) Close() error {
	if h.controller == nil {
		return nil
	}
	return h.controller.Close()
}

func (h *windowsImportHost) Attach(ctx context.Context, info DeviceInfoTruncated, conn net.Conn) (AttachedSession, error) {
	h.logger.Debug("usbip windows: attaching ", info.BusIDString(),
		fmt.Sprintf(" vid=0x%04x pid=0x%04x speed=%d", info.IDVendor, info.IDProduct, info.Speed))
	session := &windowsClientSession{
		logger:     h.logger,
		controller: h.controller,
		info:       info,
		remote:     conn,
		done:       make(chan struct{}),
	}
	err := session.start(ctx)
	if err != nil {
		return nil, err
	}
	return session, nil
}

var _ AttachedSession = (*windowsClientSession)(nil)

type windowsClientSession struct {
	logger     log.ContextLogger
	controller *usbipvhci.Controller
	info       DeviceInfoTruncated
	remote     net.Conn

	ctx    context.Context
	cancel context.CancelFunc

	listener     net.Listener
	relayService string // loopback port passed to Plugin, for StopAttachAttempts
	hubPort      int

	connAccess sync.Mutex
	driverConn net.Conn

	done      chan struct{}
	doneOnce  sync.Once
	closeOnce sync.Once

	errAccess sync.Mutex
	err       error
}

func (s *windowsClientSession) start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	listener, err := net.Listen("tcp", net.JoinHostPort(loopbackHost, "0"))
	if err != nil {
		s.cancel()
		return E.Cause(err, "usbip windows: listen loopback relay")
	}
	s.listener = listener

	// Closing the transport unblocks accept, the handshake, and the relay
	// when the session is torn down (Close, plugin failure, or ctx done).
	context.AfterFunc(s.ctx, func() {
		_ = listener.Close()
		s.connAccess.Lock()
		driverConn := s.driverConn
		s.connAccess.Unlock()
		if driverConn != nil {
			_ = driverConn.Close()
		}
		_ = s.remote.Close()
	})

	go s.acceptAndRelay()

	// Plugin blocks until the driver has connected to the loopback
	// listener and the import handshake (run by acceptAndRelay) completed.
	port := listener.Addr().(*net.TCPAddr).Port
	s.relayService = strconv.Itoa(port)
	hubPort, err := s.controller.Plugin(loopbackHost, s.relayService, s.info.BusIDString())
	if err != nil {
		s.cancel()
		s.markDone()
		return E.Cause(err, "usbip windows: vhci plugin")
	}
	s.hubPort = hubPort
	return nil
}

// acceptAndRelay accepts loopback connections until one proves to be
// the VHCI driver, then splices it to the server connection. The
// listener address is observable by any local process between Listen
// and the driver's in-kernel connect, so each accepted peer must be
// authenticated (kernel-owned socket + correct import handshake)
// before it sees device data; rejected peers do not end the session.
func (s *windowsClientSession) acceptAndRelay() {
	defer s.markDone()

	var driverConn net.Conn
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.ctx.Err() == nil {
				s.logger.Debug("usbip windows: accept vhci driver: ", err)
			}
			return
		}
		err = s.verifyDriverConn(conn)
		if err != nil {
			_ = conn.Close()
			if s.ctx.Err() != nil {
				return
			}
			s.logger.Warn("usbip windows: rejected loopback peer: ", err)
			continue
		}
		driverConn = conn
		break
	}
	_ = s.listener.Close() // one-shot: the driver has connected

	s.connAccess.Lock()
	s.driverConn = driverConn
	s.connAccess.Unlock()
	if s.ctx.Err() != nil {
		_ = driverConn.Close()
		return
	}

	relay(driverConn, s.remote)
}

// verifyDriverConn authenticates an accepted loopback connection: the
// peer socket must be owned by the kernel (the driver connects via
// WSK, attributed to the System process) and must complete the import
// handshake for exactly the device this session carries.
func (s *windowsClientSession) verifyDriverConn(conn net.Conn) error {
	pid, err := loopbackPeerPID(conn)
	if err != nil {
		return E.Cause(err, "resolve loopback peer")
	}
	if pid != systemProcessID && pid != 0 {
		return E.New("peer is process ", pid, ", not the kernel")
	}
	_ = conn.SetDeadline(time.Now().Add(relayHandshakeTimeout))
	err = s.respondImport(conn)
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Time{})
	return nil
}

// respondImport answers the driver's in-kernel OP_REQ_IMPORT from the
// cached device info, leaving both sides positioned at the data phase.
// The driver verifies the bus id in our reply equals the one it sent
// (which sing-box passed to Plugin), so info.BusID must round-trip.
func (s *windowsClientSession) respondImport(driverConn net.Conn) error {
	header, err := ReadOpHeader(driverConn)
	if err != nil {
		return E.Cause(err, "read driver OP_REQ_IMPORT header")
	}
	if header.Code != OpReqImport {
		return E.New("unexpected driver op code ", fmt.Sprintf("0x%04x", header.Code))
	}
	busid, err := ReadOpReqImportBody(driverConn)
	if err != nil {
		return E.Cause(err, "read driver OP_REQ_IMPORT body")
	}
	if busid != s.info.BusIDString() {
		return E.New("import handshake for ", busid, ", session carries ", s.info.BusIDString())
	}
	info := s.info
	err = WriteOpRepImport(driverConn, OpRepImport, OpStatusOK, &info)
	if err != nil {
		return E.Cause(err, "write OP_REP_IMPORT to driver")
	}
	return nil
}

// loopbackPeerPID resolves the owning process of the peer side of an
// accepted loopback TCP connection via GetExtendedTcpTable: the row
// whose local endpoint is our remote endpoint (and vice versa).
func loopbackPeerPID(conn net.Conn) (uint32, error) {
	remote, remoteOK := conn.RemoteAddr().(*net.TCPAddr)
	local, localOK := conn.LocalAddr().(*net.TCPAddr)
	if !remoteOK || !localOK {
		return 0, E.New("unexpected address type")
	}
	const tcpTableOwnerPIDAll = 5
	const rowSize = 24 // MIB_TCPROW_OWNER_PID
	var size uint32
	var table []byte
	for {
		var tablePtr *byte
		if len(table) > 0 {
			tablePtr = &table[0]
		}
		ret, _, _ := procGetExtendedTcpTable.Call(
			uintptr(unsafe.Pointer(tablePtr)),
			uintptr(unsafe.Pointer(&size)),
			0,
			uintptr(windows.AF_INET),
			tcpTableOwnerPIDAll,
			0,
		)
		if ret == uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
			table = make([]byte, size)
			continue
		}
		if ret != 0 {
			return 0, E.New("GetExtendedTcpTable error ", ret)
		}
		break
	}
	if len(table) < 4 {
		return 0, E.New("GetExtendedTcpTable returned no table")
	}
	count := int(binary.LittleEndian.Uint32(table[0:4]))
	for i := 0; i < count; i++ {
		row := table[4+i*rowSize:]
		if len(row) < rowSize {
			break
		}
		rowLocalIP := net.IP(row[4:8])
		rowLocalPort := int(binary.BigEndian.Uint16(row[8:10]))
		rowRemoteIP := net.IP(row[12:16])
		rowRemotePort := int(binary.BigEndian.Uint16(row[16:18]))
		if rowLocalPort == remote.Port && rowRemotePort == local.Port &&
			rowLocalIP.Equal(remote.IP) && rowRemoteIP.Equal(local.IP) {
			return binary.LittleEndian.Uint32(row[20:24]), nil
		}
	}
	return 0, E.New("peer connection not found in tcp table")
}

var (
	modIPHelper             = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable = modIPHelper.NewProc("GetExtendedTcpTable")
)

// relay splices the loopback driver stream to the proxied server stream
// until either direction ends, then closes both.
func relay(driverConn, remote net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(remote, driverConn)
		_ = remote.Close()
		_ = driverConn.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(driverConn, remote)
		_ = driverConn.Close()
		_ = remote.Close()
	}()
	wg.Wait()
}

func (s *windowsClientSession) markDone() {
	s.doneOnce.Do(func() {
		close(s.done)
	})
}

func (s *windowsClientSession) setErr(err error) {
	s.errAccess.Lock()
	if s.err == nil {
		s.err = err
	}
	s.errAccess.Unlock()
}

func (s *windowsClientSession) Done() <-chan struct{} {
	return s.done
}

func (s *windowsClientSession) Err() error {
	s.errAccess.Lock()
	defer s.errAccess.Unlock()
	return s.err
}

func (s *windowsClientSession) Start() error {
	return nil
}

func (s *windowsClientSession) Close() error {
	s.closeOnce.Do(func() {
		// If the connection already dropped, the driver has scheduled
		// background reattach attempts toward this session's dead
		// loopback port; cancel them before detaching.
		count, err := s.controller.StopAttachAttempts(loopbackHost, s.relayService, s.info.BusIDString())
		if err != nil {
			s.logger.Debug("usbip windows: stop attach attempts: ", err)
		} else if count > 0 {
			s.logger.Debug("usbip windows: canceled ", count, " driver reattach attempts")
		}
		if s.hubPort > 0 {
			err = s.controller.Plugout(s.hubPort)
			if err != nil {
				s.logger.Debug("usbip windows: plugout port ", s.hubPort, ": ", err)
			}
		}
		if s.cancel != nil {
			s.cancel()
		}
	})
	<-s.done
	return nil
}

func (s *windowsClientSession) Description() string {
	return fmt.Sprintf("usbip2_ude port %d", s.hubPort)
}
