//go:build go1.20 && !go1.21

package badtls

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"io"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"
)

type Conn struct {
	*tls.Conn
	writer              N.ExtendedWriter
	isHandshakeComplete *atomic.Bool
	activeCall          *atomic.Int32
	closeNotifySent     *bool
	version             *uint16
	rand                io.Reader
	halfAccess          *sync.Mutex
	halfError           *error
	cipher              cipher.AEAD
	explicitNonceLen    int
	halfPtr             uintptr
	halfSeq             []byte
	halfScratchBuf      []byte
}

func TryCreate(conn aTLS.Conn) aTLS.Conn {
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return conn
	}
	badConn, err := Create(tlsConn)
	if err != nil {
		log.Warn("initialize badtls: ", err)
		return conn
	}
	return badConn
}

func Create(conn *tls.Conn) (aTLS.Conn, error) {
	rawConn := reflect.Indirect(reflect.ValueOf(conn))
	rawIsHandshakeComplete := rawConn.FieldByName("isHandshakeComplete")
	if !rawIsHandshakeComplete.IsValid() || rawIsHandshakeComplete.Kind() != reflect.Struct {
		return nil, E.New("badtls: invalid isHandshakeComplete")
	}
	isHandshakeComplete := (*atomic.Bool)(unsafe.Pointer(rawIsHandshakeComplete.UnsafeAddr()))
	if !isHandshakeComplete.Load() {
		return nil, E.New("handshake not finished")
	}
	rawActiveCall := rawConn.FieldByName("activeCall")
	if !rawActiveCall.IsValid() || rawActiveCall.Kind() != reflect.Struct {
		return nil, E.New("badtls: invalid active call")
	}
	activeCall := (*atomic.Int32)(unsafe.Pointer(rawActiveCall.UnsafeAddr()))
	rawHalfConn := rawConn.FieldByName("out")
	if !rawHalfConn.IsValid() || rawHalfConn.Kind() != reflect.Struct {
		return nil, E.New("badtls: invalid half conn")
	}
	rawVersion := rawConn.FieldByName("vers")
	if !rawVersion.IsValid() || rawVersion.Kind() != reflect.Uint16 {
		return nil, E.New("badtls: invalid version")
	}
	version := (*uint16)(unsafe.Pointer(rawVersion.UnsafeAddr()))
	rawCloseNotifySent := rawConn.FieldByName("closeNotifySent")
	if !rawCloseNotifySent.IsValid() || rawCloseNotifySent.Kind() != reflect.Bool {
		return nil, E.New("badtls: invalid notify")
	}
	closeNotifySent := (*bool)(unsafe.Pointer(rawCloseNotifySent.UnsafeAddr()))
	rawConfig := reflect.Indirect(rawConn.FieldByName("config"))
	if !rawConfig.IsValid() || rawConfig.Kind() != reflect.Struct {
		return nil, E.New("badtls: bad config")
	}
	config := (*tls.Config)(unsafe.Pointer(rawConfig.UnsafeAddr()))
	randReader := config.Rand
	if randReader == nil {
		randReader = rand.Reader
	}
	rawHalfMutex := rawHalfConn.FieldByName("Mutex")
	if !rawHalfMutex.IsValid() || rawHalfMutex.Kind() != reflect.Struct {
		return nil, E.New("badtls: invalid half mutex")
	}
	halfAccess := (*sync.Mutex)(unsafe.Pointer(rawHalfMutex.UnsafeAddr()))
	rawHalfError := rawHalfConn.FieldByName("err")
	if !rawHalfError.IsValid() || rawHalfError.Kind() != reflect.Interface {
		return nil, E.New("badtls: invalid half error")
	}
	halfError := (*error)(unsafe.Pointer(rawHalfError.UnsafeAddr()))
	rawHalfCipherInterface := rawHalfConn.FieldByName("cipher")
	if !rawHalfCipherInterface.IsValid() || rawHalfCipherInterface.Kind() != reflect.Interface {
		return nil, E.New("badtls: invalid cipher interface")
	}
	rawHalfCipher := rawHalfCipherInterface.Elem()
	aeadCipher, loaded := valueInterface(rawHalfCipher, false).(cipher.AEAD)
	if !loaded {
		return nil, E.New("badtls: invalid AEAD cipher")
	}
	var explicitNonceLen int
	switch cipherName := reflect.Indirect(rawHalfCipher).Type().String(); cipherName {
	case "tls.prefixNonceAEAD":
		explicitNonceLen = aeadCipher.NonceSize()
	case "tls.xorNonceAEAD":
	default:
		return nil, E.New("badtls: unknown cipher type: ", cipherName)
	}
	rawHalfSeq := rawHalfConn.FieldByName("seq")
	if !rawHalfSeq.IsValid() || rawHalfSeq.Kind() != reflect.Array {
		return nil, E.New("badtls: invalid seq")
	}
	halfSeq := rawHalfSeq.Bytes()
	rawHalfScratchBuf := rawHalfConn.FieldByName("scratchBuf")
	if !rawHalfScratchBuf.IsValid() || rawHalfScratchBuf.Kind() != reflect.Array {
		return nil, E.New("badtls: invalid scratchBuf")
	}
	halfScratchBuf := rawHalfScratchBuf.Bytes()
	return &Conn{
		Conn:                conn,
		writer:              bufio.NewExtendedWriter(conn.NetConn()),
		isHandshakeComplete: isHandshakeComplete,
		activeCall:          activeCall,
		closeNotifySent:     closeNotifySent,
		version:             version,
		halfAccess:          halfAccess,
		halfError:           halfError,
		cipher:              aeadCipher,
		explicitNonceLen:    explicitNonceLen,
		rand:                randReader,
		halfPtr:             rawHalfConn.UnsafeAddr(),
		halfSeq:             halfSeq,
		halfScratchBuf:      halfScratchBuf,
	}, nil
}

func (c *Conn) WriteBuffer(buffer *buf.Buffer) error {
	if buffer.Len() > maxPlaintext {
		defer buffer.Release()
		return common.Error(c.Write(buffer.Bytes()))
	}
	for {
		x := c.activeCall.Load()
		if x&1 != 0 {
			return net.ErrClosed
		}
		if c.activeCall.CompareAndSwap(x, x+2) {
			break
		}
	}
	defer c.activeCall.Add(-2)
	c.halfAccess.Lock()
	defer c.halfAccess.Unlock()
	if err := *c.halfError; err != nil {
		return err
	}
	if *c.closeNotifySent {
		return errShutdown
	}
	dataLen := buffer.Len()
	dataBytes := buffer.Bytes()
	outBuf := buffer.ExtendHeader(recordHeaderLen + c.explicitNonceLen)
	outBuf[0] = 23
	version := *c.version
	if version == 0 {
		version = tls.VersionTLS10
	} else if version == tls.VersionTLS13 {
		version = tls.VersionTLS12
	}
	binary.BigEndian.PutUint16(outBuf[1:], version)
	var nonce []byte
	if c.explicitNonceLen > 0 {
		nonce = outBuf[5 : 5+c.explicitNonceLen]
		if c.explicitNonceLen < 16 {
			copy(nonce, c.halfSeq)
		} else {
			if _, err := io.ReadFull(c.rand, nonce); err != nil {
				return err
			}
		}
	}
	if len(nonce) == 0 {
		nonce = c.halfSeq
	}
	if *c.version == tls.VersionTLS13 {
		buffer.FreeBytes()[0] = 23
		binary.BigEndian.PutUint16(outBuf[3:], uint16(dataLen+1+c.cipher.Overhead()))
		c.cipher.Seal(outBuf, nonce, outBuf[recordHeaderLen:recordHeaderLen+c.explicitNonceLen+dataLen+1], outBuf[:recordHeaderLen])
		buffer.Extend(1 + c.cipher.Overhead())
	} else {
		binary.BigEndian.PutUint16(outBuf[3:], uint16(dataLen))
		additionalData := append(c.halfScratchBuf[:0], c.halfSeq...)
		additionalData = append(additionalData, outBuf[:recordHeaderLen]...)
		c.cipher.Seal(outBuf, nonce, dataBytes, additionalData)
		buffer.Extend(c.cipher.Overhead())
		binary.BigEndian.PutUint16(outBuf[3:], uint16(dataLen+c.explicitNonceLen+c.cipher.Overhead()))
	}
	incSeq(c.halfPtr)
	log.Trace("badtls write ", buffer.Len())
	return c.writer.WriteBuffer(buffer)
}

func (c *Conn) FrontHeadroom() int {
	return recordHeaderLen + c.explicitNonceLen
}

func (c *Conn) RearHeadroom() int {
	return 1 + c.cipher.Overhead()
}

func (c *Conn) WriterMTU() int {
	return maxPlaintext
}

func (c *Conn) Upstream() any {
	return c.Conn
}

func (c *Conn) UpstreamWriter() any {
	return c.NetConn()
}
