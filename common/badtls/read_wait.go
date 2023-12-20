//go:build go1.21 && !without_badtls

package badtls

import (
	"bytes"
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
	*tls.STDConn
	halfAccess      *sync.Mutex
	rawInput        *bytes.Buffer
	input           *bytes.Reader
	hand            *bytes.Buffer
	readWaitOptions N.ReadWaitOptions
}

func NewReadWaitConn(conn tls.Conn) (tls.Conn, error) {
	stdConn, isSTDConn := conn.(*tls.STDConn)
	if !isSTDConn {
		return nil, os.ErrInvalid
	}
	rawConn := reflect.Indirect(reflect.ValueOf(stdConn))
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
		STDConn:    stdConn,
		halfAccess: halfAccess,
		rawInput:   rawInput,
		input:      input,
		hand:       hand,
	}, nil
}

func (c *ReadWaitConn) InitializeReadWaiter(options N.ReadWaitOptions) (needCopy bool) {
	c.readWaitOptions = options
	return false
}

func (c *ReadWaitConn) WaitReadBuffer() (buffer *buf.Buffer, err error) {
	err = c.Handshake()
	if err != nil {
		return
	}
	c.halfAccess.Lock()
	defer c.halfAccess.Unlock()
	for c.input.Len() == 0 {
		err = tlsReadRecord(c.STDConn)
		if err != nil {
			return
		}
		for c.hand.Len() > 0 {
			err = tlsHandlePostHandshakeMessage(c.STDConn)
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
		_ = tlsReadRecord(c.STDConn)
		// return n, err // will be io.EOF on closeNotify
	}

	c.readWaitOptions.PostReturn(buffer)
	return
}

//go:linkname tlsReadRecord crypto/tls.(*Conn).readRecord
func tlsReadRecord(c *tls.STDConn) error

//go:linkname tlsHandlePostHandshakeMessage crypto/tls.(*Conn).handlePostHandshakeMessage
func tlsHandlePostHandshakeMessage(c *tls.STDConn) error
