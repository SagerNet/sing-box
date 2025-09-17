//go:build linux && go1.25 && badlinkname

package ktls

import (
	"crypto/tls"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/sagernet/sing-box/common/badversion"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/shell"

	"golang.org/x/sys/unix"
)

// mod from https://gitlab.com/go-extension/tls

const (
	TLS_TX               = 1
	TLS_RX               = 2
	TLS_TX_ZEROCOPY_RO   = 3 // TX zerocopy (only sendfile now)
	TLS_RX_EXPECT_NO_PAD = 4 // Attempt opportunistic zero-copy, TLS 1.3 only

	TLS_SET_RECORD_TYPE = 1
	TLS_GET_RECORD_TYPE = 2
)

type Support struct {
	TLS, TLS_RX                     bool
	TLS_Version13, TLS_Version13_RX bool

	TLS_TX_ZEROCOPY  bool
	TLS_RX_NOPADDING bool

	TLS_AES_256_GCM       bool
	TLS_AES_128_CCM       bool
	TLS_CHACHA20_POLY1305 bool
	TLS_SM4               bool
	TLS_ARIA_GCM          bool

	TLS_Version13_KeyUpdate bool
}

var KernelSupport = sync.OnceValues(func() (*Support, error) {
	var uname unix.Utsname
	err := unix.Uname(&uname)
	if err != nil {
		return nil, err
	}

	kernelVersion := badversion.Parse(strings.Trim(string(uname.Release[:]), "\x00"))
	if err != nil {
		return nil, err
	}
	var support Support
	switch {
	case kernelVersion.GreaterThanOrEqual(badversion.Version{Major: 6, Minor: 14}):
		support.TLS_Version13_KeyUpdate = true
		fallthrough
	case kernelVersion.GreaterThanOrEqual(badversion.Version{Major: 6, Minor: 1}):
		support.TLS_ARIA_GCM = true
		fallthrough
	case kernelVersion.GreaterThanOrEqual(badversion.Version{Major: 6}):
		support.TLS_Version13_RX = true
		support.TLS_RX_NOPADDING = true
		fallthrough
	case kernelVersion.GreaterThanOrEqual(badversion.Version{Major: 5, Minor: 19}):
		support.TLS_TX_ZEROCOPY = true
		fallthrough
	case kernelVersion.GreaterThanOrEqual(badversion.Version{Major: 5, Minor: 16}):
		support.TLS_SM4 = true
		fallthrough
	case kernelVersion.GreaterThanOrEqual(badversion.Version{Major: 5, Minor: 11}):
		support.TLS_CHACHA20_POLY1305 = true
		fallthrough
	case kernelVersion.GreaterThanOrEqual(badversion.Version{Major: 5, Minor: 2}):
		support.TLS_AES_128_CCM = true
		fallthrough
	case kernelVersion.GreaterThanOrEqual(badversion.Version{Major: 5, Minor: 1}):
		support.TLS_AES_256_GCM = true
		support.TLS_Version13 = true
		fallthrough
	case kernelVersion.GreaterThanOrEqual(badversion.Version{Major: 4, Minor: 17}):
		support.TLS_RX = true
		fallthrough
	case kernelVersion.GreaterThanOrEqual(badversion.Version{Major: 4, Minor: 13}):
		support.TLS = true
	}

	if support.TLS && support.TLS_Version13 {
		_, err := os.Stat("/sys/module/tls")
		if err != nil {
			if os.Getuid() == 0 {
				output, err := shell.Exec("modprobe", "tls").Read()
				if err != nil {
					return nil, E.Extend(E.Cause(err, "modprobe tls"), output)
				}
			} else {
				return nil, E.New("ktls: kernel TLS module not loaded")
			}
		}
	}

	return &support, nil
})

func Load() error {
	support, err := KernelSupport()
	if err != nil {
		return E.Cause(err, "ktls: check availability")
	}
	if !support.TLS || !support.TLS_Version13 {
		return E.New("ktls: kernel does not support TLS 1.3")
	}
	return nil
}

func (c *Conn) setupKernel(txOffload, rxOffload bool) error {
	if !txOffload && !rxOffload {
		return os.ErrInvalid
	}
	support, err := KernelSupport()
	if err != nil {
		return E.Cause(err, "check availability")
	}
	if !support.TLS || !support.TLS_Version13 {
		return E.New("kernel does not support TLS 1.3")
	}
	c.rawConn.Out.Lock()
	defer c.rawConn.Out.Unlock()
	err = control.Raw(c.rawSyscallConn, func(fd uintptr) error {
		return syscall.SetsockoptString(int(fd), unix.SOL_TCP, unix.TCP_ULP, "tls")
	})
	if err != nil {
		return os.NewSyscallError("setsockopt", err)
	}

	if txOffload {
		txCrypto := kernelCipher(support, c.rawConn.Out, *c.rawConn.CipherSuite, false)
		if txCrypto == nil {
			return E.New("unsupported cipher suite")
		}
		err = control.Raw(c.rawSyscallConn, func(fd uintptr) error {
			return syscall.SetsockoptString(int(fd), unix.SOL_TLS, TLS_TX, txCrypto.String())
		})
		if err != nil {
			return err
		}
		if support.TLS_TX_ZEROCOPY {
			err = control.Raw(c.rawSyscallConn, func(fd uintptr) error {
				return syscall.SetsockoptInt(int(fd), unix.SOL_TLS, TLS_TX_ZEROCOPY_RO, 1)
			})
			if err != nil {
				return err
			}
		}
		c.kernelTx = true
		c.logger.DebugContext(c.ctx, "ktls: kernel TLS TX enabled")
	}

	if rxOffload {
		rxCrypto := kernelCipher(support, c.rawConn.In, *c.rawConn.CipherSuite, true)
		if rxCrypto == nil {
			return E.New("unsupported cipher suite")
		}
		err = control.Raw(c.rawSyscallConn, func(fd uintptr) error {
			return syscall.SetsockoptString(int(fd), unix.SOL_TLS, TLS_RX, rxCrypto.String())
		})
		if err != nil {
			return err
		}
		if *c.rawConn.Vers >= tls.VersionTLS13 && support.TLS_RX_NOPADDING {
			err = control.Raw(c.rawSyscallConn, func(fd uintptr) error {
				return syscall.SetsockoptInt(int(fd), unix.SOL_TLS, TLS_RX_EXPECT_NO_PAD, 1)
			})
			if err != nil {
				return err
			}
		}
		c.kernelRx = true
		c.logger.DebugContext(c.ctx, "ktls: kernel TLS RX enabled")
	}
	return nil
}

func (c *Conn) resetupTX() (func() error, error) {
	if !c.kernelTx {
		return nil, nil
	}
	support, err := KernelSupport()
	if err != nil {
		return nil, err
	}
	if !support.TLS_Version13_KeyUpdate {
		return nil, errors.New("ktls: kernel does not support rekey")
	}
	txCrypto := kernelCipher(support, c.rawConn.Out, *c.rawConn.CipherSuite, false)
	if txCrypto == nil {
		return nil, errors.New("ktls: set kernelCipher on unsupported tls session")
	}
	return func() error {
		return control.Raw(c.rawSyscallConn, func(fd uintptr) error {
			return syscall.SetsockoptString(int(fd), unix.SOL_TLS, TLS_TX, txCrypto.String())
		})
	}, nil
}

func (c *Conn) resetupRX() error {
	if !c.kernelRx {
		return nil
	}
	support, err := KernelSupport()
	if err != nil {
		return err
	}
	if !support.TLS_Version13_KeyUpdate {
		return errors.New("ktls: kernel does not support rekey")
	}
	rxCrypto := kernelCipher(support, c.rawConn.In, *c.rawConn.CipherSuite, true)
	if rxCrypto == nil {
		return errors.New("ktls: set kernelCipher on unsupported tls session")
	}
	return control.Raw(c.rawSyscallConn, func(fd uintptr) error {
		return syscall.SetsockoptString(int(fd), unix.SOL_TLS, TLS_RX, rxCrypto.String())
	})
}

func (c *Conn) readKernelRecord() (uint8, []byte, error) {
	if c.rawConn.RawInput.Len() < maxPlaintext {
		c.rawConn.RawInput.Grow(maxPlaintext - c.rawConn.RawInput.Len())
	}

	data := c.rawConn.RawInput.Bytes()[:maxPlaintext]

	// cmsg for record type
	buffer := make([]byte, unix.CmsgSpace(1))
	cmsg := (*unix.Cmsghdr)(unsafe.Pointer(&buffer[0]))
	cmsg.SetLen(unix.CmsgLen(1))

	var iov unix.Iovec
	iov.Base = &data[0]
	iov.SetLen(len(data))

	var msg unix.Msghdr
	msg.Control = &buffer[0]
	msg.Controllen = cmsg.Len
	msg.Iov = &iov
	msg.Iovlen = 1

	var n int
	var err error
	er := c.rawSyscallConn.Read(func(fd uintptr) bool {
		n, err = recvmsg(int(fd), &msg, 0)
		return err != unix.EAGAIN || c.pendingRxSplice
	})
	if er != nil {
		return 0, nil, er
	}
	switch err {
	case nil:
	case syscall.EINVAL, syscall.EAGAIN:
		return 0, nil, c.rawConn.In.SetErrorLocked(c.sendAlert(alertProtocolVersion))
	case syscall.EMSGSIZE:
		return 0, nil, c.rawConn.In.SetErrorLocked(c.sendAlert(alertRecordOverflow))
	case syscall.EBADMSG:
		return 0, nil, c.rawConn.In.SetErrorLocked(c.sendAlert(alertDecryptError))
	default:
		return 0, nil, err
	}

	if n <= 0 {
		return 0, nil, c.rawConn.In.SetErrorLocked(io.EOF)
	}

	if cmsg.Level == unix.SOL_TLS && cmsg.Type == TLS_GET_RECORD_TYPE {
		typ := buffer[unix.CmsgLen(0)]
		return typ, data[:n], nil
	}

	return recordTypeApplicationData, data[:n], nil
}

func (c *Conn) writeKernelRecord(typ uint16, data []byte) (int, error) {
	if typ == recordTypeApplicationData {
		return c.conn.Write(data)
	}

	// cmsg for record type
	buffer := make([]byte, unix.CmsgSpace(1))
	cmsg := (*unix.Cmsghdr)(unsafe.Pointer(&buffer[0]))
	cmsg.SetLen(unix.CmsgLen(1))
	buffer[unix.CmsgLen(0)] = byte(typ)
	cmsg.Level = unix.SOL_TLS
	cmsg.Type = TLS_SET_RECORD_TYPE

	var iov unix.Iovec
	iov.Base = &data[0]
	iov.SetLen(len(data))

	var msg unix.Msghdr
	msg.Control = &buffer[0]
	msg.Controllen = cmsg.Len
	msg.Iov = &iov
	msg.Iovlen = 1

	var n int
	var err error
	ew := c.rawSyscallConn.Write(func(fd uintptr) bool {
		n, err = sendmsg(int(fd), &msg, 0)
		return err != unix.EAGAIN
	})
	if ew != nil {
		return 0, ew
	}
	return n, err
}

//go:linkname recvmsg golang.org/x/sys/unix.recvmsg
func recvmsg(fd int, msg *unix.Msghdr, flags int) (n int, err error)

//go:linkname sendmsg golang.org/x/sys/unix.sendmsg
func sendmsg(fd int, msg *unix.Msghdr, flags int) (n int, err error)
