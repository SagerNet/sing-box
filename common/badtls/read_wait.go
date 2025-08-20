//go:build go1.21 && !without_badtls

package badtls

import (
	"bytes"
	"context"
	"net"
	"os"
	"reflect"
	"sync"
	"unsafe"

	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/tls"
)

var _ N.ReadWaiter = (*ReadWaitConn)(nil)

type ReadWaitConn struct {
	tls.Conn
	halfAccess                    *sync.Mutex
	rawInput                      *bytes.Buffer
	input                         *bytes.Reader
	hand                          *bytes.Buffer
	readWaitOptions               N.ReadWaitOptions
	tlsReadRecord                 func() error
	tlsHandlePostHandshakeMessage func() error
}

func NewReadWaitConn(conn tls.Conn) (tls.Conn, error) {
	var (
		loaded                        bool
		tlsReadRecord                 func() error
		tlsHandlePostHandshakeMessage func() error
	)
	for _, tlsCreator := range tlsRegistry {
		loaded, tlsReadRecord, tlsHandlePostHandshakeMessage = tlsCreator(conn)
		if loaded {
			break
		}
	}
	if !loaded {
		return nil, os.ErrInvalid
	}
	rawConn := reflect.Indirect(reflect.ValueOf(conn))
	rawHalfConn := rawConn.FieldByName("in")
	if !rawHalfConn.IsValid() || rawHalfConn.Kind() != reflect.Struct {
		return nil, E.New("badtls: invalid half conn")
	}
	rawHalfMutex := rawHalfConn.FieldByName("Mutex")
	if !rawHalfMutex.IsValid() || rawHalfMutex.Kind() != reflect.Struct {
		return nil, E.New("badtls: invalid half mutex")
	}
	halfAccess := (*sync.Mutex)(unsafe.Pointer(rawHalfMutex.UnsafeAddr()))
	rawRawInput := rawConn.FieldByName("rawInput")
	if !rawRawInput.IsValid() || rawRawInput.Kind() != reflect.Struct {
		return nil, E.New("badtls: invalid raw input")
	}
	rawInput := (*bytes.Buffer)(unsafe.Pointer(rawRawInput.UnsafeAddr()))
	rawInput0 := rawConn.FieldByName("input")
	if !rawInput0.IsValid() || rawInput0.Kind() != reflect.Struct {
		return nil, E.New("badtls: invalid input")
	}
	input := (*bytes.Reader)(unsafe.Pointer(rawInput0.UnsafeAddr()))
	rawHand := rawConn.FieldByName("hand")
	if !rawHand.IsValid() || rawHand.Kind() != reflect.Struct {
		return nil, E.New("badtls: invalid hand")
	}
	hand := (*bytes.Buffer)(unsafe.Pointer(rawHand.UnsafeAddr()))
	return &ReadWaitConn{
		Conn:                          conn,
		halfAccess:                    halfAccess,
		rawInput:                      rawInput,
		input:                         input,
		hand:                          hand,
		tlsReadRecord:                 tlsReadRecord,
		tlsHandlePostHandshakeMessage: tlsHandlePostHandshakeMessage,
	}, nil
}

func (c *ReadWaitConn) InitializeReadWaiter(options N.ReadWaitOptions) (needCopy bool) {
	c.readWaitOptions = options
	return false
}

func (c *ReadWaitConn) WaitReadBuffer() (buffer *buf.Buffer, err error) {
	err = c.HandshakeContext(context.Background())
	if err != nil {
		return
	}
	c.halfAccess.Lock()
	defer c.halfAccess.Unlock()
	for c.input.Len() == 0 {
		err = c.tlsReadRecord()
		if err != nil {
			return
		}
		for c.hand.Len() > 0 {
			err = c.tlsHandlePostHandshakeMessage()
			if err != nil {
				return
			}
		}
	}
	buffer = c.readWaitOptions.NewBuffer()
	n, err := c.input.Read(buffer.FreeBytes())
	if err != nil {
		buffer.Release()
		return
	}
	buffer.Truncate(n)

	if n != 0 && c.input.Len() == 0 && c.rawInput.Len() > 0 &&
		// recordType(c.rawInput.Bytes()[0]) == recordTypeAlert {
		c.rawInput.Bytes()[0] == 21 {
		_ = c.tlsReadRecord()
		// return n, err // will be io.EOF on closeNotify
	}

	c.readWaitOptions.PostReturn(buffer)
	return
}

func (c *ReadWaitConn) Upstream() any {
	return c.Conn
}

func (c *ReadWaitConn) ReaderReplaceable() bool {
	return true
}

var tlsRegistry []func(conn net.Conn) (loaded bool, tlsReadRecord func() error, tlsHandlePostHandshakeMessage func() error)

func init() {
	tlsRegistry = append(tlsRegistry, func(conn net.Conn) (loaded bool, tlsReadRecord func() error, tlsHandlePostHandshakeMessage func() error) {
		tlsConn, loaded := conn.(*tls.STDConn)
		if !loaded {
			return
		}
		return true, func() error {
				return stdTLSReadRecord(tlsConn)
			}, func() error {
				return stdTLSHandlePostHandshakeMessage(tlsConn)
			}
	})
}

//go:linkname stdTLSReadRecord crypto/tls.(*Conn).readRecord
func stdTLSReadRecord(c *tls.STDConn) error

//go:linkname stdTLSHandlePostHandshakeMessage crypto/tls.(*Conn).handlePostHandshakeMessage
func stdTLSHandlePostHandshakeMessage(c *tls.STDConn) error
