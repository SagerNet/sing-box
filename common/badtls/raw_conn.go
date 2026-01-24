//go:build go1.25 && badlinkname

package badtls

import (
	"bytes"
	"os"
	"reflect"
	"sync/atomic"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/tls"
)

type RawConn struct {
	pointer unsafe.Pointer
	methods *Methods

	IsClient            *bool
	IsHandshakeComplete *atomic.Bool
	Vers                *uint16
	CipherSuite         *uint16

	RawInput *bytes.Buffer
	Input    *bytes.Reader
	Hand     *bytes.Buffer

	CloseNotifySent *bool
	CloseNotifyErr  *error

	In  *RawHalfConn
	Out *RawHalfConn

	BytesSent   *int64
	PacketsSent *int64

	ActiveCall *atomic.Int32
	Tmp        *[16]byte
}

func NewRawConn(rawTLSConn tls.Conn) (*RawConn, error) {
	var (
		pointer unsafe.Pointer
		methods *Methods
		loaded  bool
	)
	for _, tlsCreator := range methodRegistry {
		pointer, methods, loaded = tlsCreator(rawTLSConn)
		if loaded {
			break
		}
	}
	if !loaded {
		return nil, os.ErrInvalid
	}

	conn := &RawConn{
		pointer: pointer,
		methods: methods,
	}

	rawConn := reflect.Indirect(reflect.ValueOf(rawTLSConn))

	rawIsClient := rawConn.FieldByName("isClient")
	if !rawIsClient.IsValid() || rawIsClient.Kind() != reflect.Bool {
		return nil, E.New("invalid Conn.isClient")
	}
	conn.IsClient = (*bool)(unsafe.Pointer(rawIsClient.UnsafeAddr()))

	rawIsHandshakeComplete := rawConn.FieldByName("isHandshakeComplete")
	if !rawIsHandshakeComplete.IsValid() || rawIsHandshakeComplete.Kind() != reflect.Struct {
		return nil, E.New("invalid Conn.isHandshakeComplete")
	}
	conn.IsHandshakeComplete = (*atomic.Bool)(unsafe.Pointer(rawIsHandshakeComplete.UnsafeAddr()))

	rawVers := rawConn.FieldByName("vers")
	if !rawVers.IsValid() || rawVers.Kind() != reflect.Uint16 {
		return nil, E.New("invalid Conn.vers")
	}
	conn.Vers = (*uint16)(unsafe.Pointer(rawVers.UnsafeAddr()))

	rawCipherSuite := rawConn.FieldByName("cipherSuite")
	if !rawCipherSuite.IsValid() || rawCipherSuite.Kind() != reflect.Uint16 {
		return nil, E.New("invalid Conn.cipherSuite")
	}
	conn.CipherSuite = (*uint16)(unsafe.Pointer(rawCipherSuite.UnsafeAddr()))

	rawRawInput := rawConn.FieldByName("rawInput")
	if !rawRawInput.IsValid() || rawRawInput.Kind() != reflect.Struct {
		return nil, E.New("invalid Conn.rawInput")
	}
	conn.RawInput = (*bytes.Buffer)(unsafe.Pointer(rawRawInput.UnsafeAddr()))

	rawInput := rawConn.FieldByName("input")
	if !rawInput.IsValid() || rawInput.Kind() != reflect.Struct {
		return nil, E.New("invalid Conn.input")
	}
	conn.Input = (*bytes.Reader)(unsafe.Pointer(rawInput.UnsafeAddr()))

	rawHand := rawConn.FieldByName("hand")
	if !rawHand.IsValid() || rawHand.Kind() != reflect.Struct {
		return nil, E.New("invalid Conn.hand")
	}
	conn.Hand = (*bytes.Buffer)(unsafe.Pointer(rawHand.UnsafeAddr()))

	rawCloseNotifySent := rawConn.FieldByName("closeNotifySent")
	if !rawCloseNotifySent.IsValid() || rawCloseNotifySent.Kind() != reflect.Bool {
		return nil, E.New("invalid Conn.closeNotifySent")
	}
	conn.CloseNotifySent = (*bool)(unsafe.Pointer(rawCloseNotifySent.UnsafeAddr()))

	rawCloseNotifyErr := rawConn.FieldByName("closeNotifyErr")
	if !rawCloseNotifyErr.IsValid() || rawCloseNotifyErr.Kind() != reflect.Interface {
		return nil, E.New("invalid Conn.closeNotifyErr")
	}
	conn.CloseNotifyErr = (*error)(unsafe.Pointer(rawCloseNotifyErr.UnsafeAddr()))

	rawIn := rawConn.FieldByName("in")
	if !rawIn.IsValid() || rawIn.Kind() != reflect.Struct {
		return nil, E.New("invalid Conn.in")
	}
	halfIn, err := NewRawHalfConn(rawIn, methods)
	if err != nil {
		return nil, E.Cause(err, "invalid Conn.in")
	}
	conn.In = halfIn

	rawOut := rawConn.FieldByName("out")
	if !rawOut.IsValid() || rawOut.Kind() != reflect.Struct {
		return nil, E.New("invalid Conn.out")
	}
	halfOut, err := NewRawHalfConn(rawOut, methods)
	if err != nil {
		return nil, E.Cause(err, "invalid Conn.out")
	}
	conn.Out = halfOut

	rawBytesSent := rawConn.FieldByName("bytesSent")
	if !rawBytesSent.IsValid() || rawBytesSent.Kind() != reflect.Int64 {
		return nil, E.New("invalid Conn.bytesSent")
	}
	conn.BytesSent = (*int64)(unsafe.Pointer(rawBytesSent.UnsafeAddr()))

	rawPacketsSent := rawConn.FieldByName("packetsSent")
	if !rawPacketsSent.IsValid() || rawPacketsSent.Kind() != reflect.Int64 {
		return nil, E.New("invalid Conn.packetsSent")
	}
	conn.PacketsSent = (*int64)(unsafe.Pointer(rawPacketsSent.UnsafeAddr()))

	rawActiveCall := rawConn.FieldByName("activeCall")
	if !rawActiveCall.IsValid() || rawActiveCall.Kind() != reflect.Struct {
		return nil, E.New("invalid Conn.activeCall")
	}
	conn.ActiveCall = (*atomic.Int32)(unsafe.Pointer(rawActiveCall.UnsafeAddr()))

	rawTmp := rawConn.FieldByName("tmp")
	if !rawTmp.IsValid() || rawTmp.Kind() != reflect.Array || rawTmp.Len() != 16 || rawTmp.Type().Elem().Kind() != reflect.Uint8 {
		return nil, E.New("invalid Conn.tmp")
	}
	conn.Tmp = (*[16]byte)(unsafe.Pointer(rawTmp.UnsafeAddr()))

	return conn, nil
}

func (c *RawConn) ReadRecord() error {
	return c.methods.readRecord(c.pointer)
}

func (c *RawConn) HandlePostHandshakeMessage() error {
	return c.methods.handlePostHandshakeMessage(c.pointer)
}

func (c *RawConn) WriteRecordLocked(typ uint16, data []byte) (int, error) {
	return c.methods.writeRecordLocked(c.pointer, typ, data)
}
