//go:build linux

package usbip

import (
	"context"
	"net"
	"os"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	sBufio "github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/sys/unix"
)

type usbipConnHandoff struct {
	conn      net.Conn
	file      *os.File
	relayConn net.Conn
}

func newUSBIPConnHandoff(conn net.Conn) (*usbipConnHandoff, error) {
	if tcpConn, _ := N.UnwrapReader(conn).(*net.TCPConn); tcpConn != nil {
		file, err := tcpConn.File()
		if err != nil {
			return nil, E.Cause(err, "dup TCP socket fd")
		}
		return &usbipConnHandoff{
			conn: conn,
			file: file,
		}, nil
	}

	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, E.Cause(err, "create USB/IP relay socketpair")
	}
	kernelFile := os.NewFile(uintptr(fds[0]), "usbip-kernel")
	relayFile := os.NewFile(uintptr(fds[1]), "usbip-relay")
	relayConn, err := net.FileConn(relayFile)
	_ = relayFile.Close()
	if err != nil {
		_ = kernelFile.Close()
		return nil, E.Cause(err, "wrap USB/IP relay socket")
	}
	return &usbipConnHandoff{
		conn:      conn,
		file:      kernelFile,
		relayConn: relayConn,
	}, nil
}

func (h *usbipConnHandoff) kernelFD() uintptr {
	return h.file.Fd()
}

func (h *usbipConnHandoff) relay() bool {
	return h.relayConn != nil
}

func (h *usbipConnHandoff) mode() string {
	if h.relay() {
		return "relay"
	}
	return "direct"
}

func (h *usbipConnHandoff) closeKernelFD() error {
	if h.file == nil {
		return nil
	}
	err := h.file.Close()
	h.file = nil
	return err
}

func (h *usbipConnHandoff) Close() error {
	return E.Errors(
		h.closeKernelFD(),
		common.Close(h.relayConn),
	)
}

func (h *usbipConnHandoff) startRelay(ctx context.Context, logger log.ContextLogger, side string, busid string) bool {
	if !h.relay() {
		err := h.conn.Close()
		if err != nil && !E.IsClosedOrCanceled(err) {
			logger.Debug("close usbip ", side, " userspace socket ", busid, ": ", err)
		}
		return true
	}
	relayConn := h.relayConn
	h.relayConn = nil
	go func() {
		err := sBufio.CopyConn(ctx, h.conn, relayConn)
		if err == nil {
			logger.Debug("usbip ", side, " relay ", busid, " closed")
		} else if ctx.Err() == nil && !E.IsClosedOrCanceled(err) {
			logger.Warn("usbip ", side, " relay ", busid, ": ", err)
		} else {
			logger.Debug("usbip ", side, " relay ", busid, ": ", err)
		}
	}()
	return true
}
