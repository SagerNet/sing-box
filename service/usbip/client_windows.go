//go:build windows

package usbip

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/sagernet/sing-box/common/usbipvhci"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

const loopbackHost = "127.0.0.1"

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

	listener net.Listener
	hubPort  int

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
	hubPort, err := s.controller.Plugin(loopbackHost, strconv.Itoa(port), s.info.BusIDString())
	if err != nil {
		s.cancel()
		s.markDone()
		return E.Cause(err, "usbip windows: vhci plugin")
	}
	s.hubPort = hubPort
	return nil
}

func (s *windowsClientSession) acceptAndRelay() {
	defer s.markDone()

	driverConn, err := s.listener.Accept()
	_ = s.listener.Close() // one-shot: only the driver should connect
	if err != nil {
		if s.ctx.Err() == nil {
			s.logger.Debug("usbip windows: accept vhci driver: ", err)
		}
		return
	}
	s.connAccess.Lock()
	s.driverConn = driverConn
	s.connAccess.Unlock()
	if s.ctx.Err() != nil {
		_ = driverConn.Close()
		return
	}

	err = s.respondImport(driverConn)
	if err != nil {
		s.setErr(err)
		if s.ctx.Err() == nil {
			s.logger.Debug("usbip windows: import handshake: ", err)
		}
		_ = driverConn.Close()
		_ = s.remote.Close()
		return
	}

	relay(driverConn, s.remote)
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
	_, err = ReadOpReqImportBody(driverConn)
	if err != nil {
		return E.Cause(err, "read driver OP_REQ_IMPORT body")
	}
	info := s.info
	err = WriteOpRepImport(driverConn, OpRepImport, OpStatusOK, &info)
	if err != nil {
		return E.Cause(err, "write OP_REP_IMPORT to driver")
	}
	return nil
}

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
		if s.hubPort > 0 {
			err := s.controller.Plugout(s.hubPort)
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
