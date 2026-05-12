//go:build linux

package usbip

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	sBufio "github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/sys/unix"
)

var _ DataSession = (*kernelHandoffSession)(nil)

// kernelHandoffSession is the Linux DataSession that bridges a USB/IP
// wire connection into the kernel's vhci_hcd / usbip-host. After the
// caller writes kernelFD() into usbip_sockfd, Start kicks off the
// monitor/relay goroutine; Done fires when the kernel detaches.
type kernelHandoffSession struct {
	conn        net.Conn
	file        *os.File
	monitorFile *os.File
	relayConn   net.Conn

	done      chan struct{}
	doneOnce  sync.Once
	runErr    error
	closeOnce sync.Once
	closeErr  error
}

func newKernelHandoffSession(conn net.Conn) (*kernelHandoffSession, error) {
	if tcpConn, _ := N.UnwrapReader(conn).(*net.TCPConn); tcpConn != nil {
		file, err := tcpConn.File()
		if err != nil {
			return nil, E.Cause(err, "dup TCP socket fd")
		}
		monitorFile, err := tcpConn.File()
		if err != nil {
			_ = file.Close()
			return nil, E.Cause(err, "dup TCP socket monitor fd")
		}
		return &kernelHandoffSession{
			conn:        conn,
			file:        file,
			monitorFile: monitorFile,
			done:        make(chan struct{}),
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
	return &kernelHandoffSession{
		conn:      conn,
		file:      kernelFile,
		relayConn: relayConn,
		done:      make(chan struct{}),
	}, nil
}

func (h *kernelHandoffSession) kernelFD() uintptr {
	return h.file.Fd()
}

func (h *kernelHandoffSession) relay() bool {
	return h.relayConn != nil
}

func (h *kernelHandoffSession) mode() string {
	if h.relay() {
		return "relay"
	}
	return "direct"
}

func (h *kernelHandoffSession) closeKernelFD() error {
	if h.file == nil {
		return nil
	}
	err := h.file.Close()
	h.file = nil
	return err
}

func (h *kernelHandoffSession) Done() <-chan struct{} {
	return h.done
}

func (h *kernelHandoffSession) Err() error {
	return h.runErr
}

func (h *kernelHandoffSession) Close() error {
	h.closeOnce.Do(func() {
		h.closeErr = E.Errors(
			h.closeKernelFD(),
			common.Close(h.monitorFile),
			common.Close(h.relayConn),
		)
		h.monitorFile = nil
		h.relayConn = nil
	})
	h.markDone(nil)
	return h.closeErr
}

func (h *kernelHandoffSession) markDone(err error) {
	h.doneOnce.Do(func() {
		h.runErr = err
		close(h.done)
	})
}

// Start launches the monitor (direct mode) or relay (userspace mode)
// goroutine. Done() closes when the goroutine exits; Err() returns the
// recorded termination cause (nil for clean detach or canceled context).
func (h *kernelHandoffSession) Start(ctx context.Context, logger log.ContextLogger, side string, busid string) {
	if !h.relay() {
		err := h.conn.Close()
		if err != nil && !E.IsClosedOrCanceled(err) {
			logger.Debug("close usbip ", side, " userspace socket ", busid, ": ", err)
		}
		monitorFile := h.monitorFile
		h.monitorFile = nil
		go h.runDirect(ctx, logger, side, busid, monitorFile)
		return
	}
	relayConn := h.relayConn
	h.relayConn = nil
	go h.runRelay(ctx, logger, side, busid, relayConn)
}

func (h *kernelHandoffSession) runDirect(ctx context.Context, logger log.ContextLogger, side string, busid string, file *os.File) {
	err := pollDirectHandoff(ctx, logger, side, busid, file)
	h.markDone(err)
}

func (h *kernelHandoffSession) runRelay(ctx context.Context, logger log.ContextLogger, side string, busid string, relayConn net.Conn) {
	err := sBufio.CopyConn(ctx, h.conn, relayConn)
	var runErr error
	switch {
	case err == nil:
		logger.Debug("usbip ", side, " relay ", busid, " closed")
	case ctx.Err() == nil && !E.IsClosedOrCanceled(err):
		logger.Warn("usbip ", side, " relay ", busid, ": ", err)
		runErr = err
	default:
		logger.Debug("usbip ", side, " relay ", busid, ": ", err)
	}
	h.markDone(runErr)
}

func pollDirectHandoff(ctx context.Context, logger log.ContextLogger, side string, busid string, file *os.File) error {
	if file == nil {
		return nil
	}
	closeFile := sync.OnceFunc(func() {
		_ = file.Close()
	})
	stopCloseOnCancel := context.AfterFunc(ctx, closeFile)
	defer func() {
		stopCloseOnCancel()
		closeFile()
	}()
	fd := int32(file.Fd())
	for {
		events := int16(unix.POLLHUP | unix.POLLERR | unix.POLLRDHUP)
		fds := []unix.PollFd{{Fd: fd, Events: events}}
		_, err := unix.Poll(fds, -1)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, unix.EBADF) {
				logger.Debug("usbip ", side, " direct monitor ", busid, ": ", err)
				return err
			}
			return nil
		}
		if fds[0].Revents&(events|unix.POLLNVAL) != 0 {
			return nil
		}
	}
}
