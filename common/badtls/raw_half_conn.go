//go:build go1.25 && badlinkname

package badtls

import (
	"hash"
	"reflect"
	"sync"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"
)

type RawHalfConn struct {
	pointer unsafe.Pointer
	methods *Methods
	*sync.Mutex
	Err           *error
	Version       *uint16
	Cipher        *any
	Seq           *[8]byte
	ScratchBuf    *[13]byte
	TrafficSecret *[]byte
	Mac           *hash.Hash
	RawKey        *[]byte
	RawIV         *[]byte
	RawMac        *[]byte
}

func NewRawHalfConn(rawHalfConn reflect.Value, methods *Methods) (*RawHalfConn, error) {
	halfConn := &RawHalfConn{
		pointer: (unsafe.Pointer)(rawHalfConn.UnsafeAddr()),
		methods: methods,
	}

	rawMutex := rawHalfConn.FieldByName("Mutex")
	if !rawMutex.IsValid() || rawMutex.Kind() != reflect.Struct {
		return nil, E.New("badtls: invalid halfConn.Mutex")
	}
	halfConn.Mutex = (*sync.Mutex)(unsafe.Pointer(rawMutex.UnsafeAddr()))

	rawErr := rawHalfConn.FieldByName("err")
	if !rawErr.IsValid() || rawErr.Kind() != reflect.Interface {
		return nil, E.New("badtls: invalid halfConn.err")
	}
	halfConn.Err = (*error)(unsafe.Pointer(rawErr.UnsafeAddr()))

	rawVersion := rawHalfConn.FieldByName("version")
	if !rawVersion.IsValid() || rawVersion.Kind() != reflect.Uint16 {
		return nil, E.New("badtls: invalid halfConn.version")
	}
	halfConn.Version = (*uint16)(unsafe.Pointer(rawVersion.UnsafeAddr()))

	rawCipher := rawHalfConn.FieldByName("cipher")
	if !rawCipher.IsValid() || rawCipher.Kind() != reflect.Interface {
		return nil, E.New("badtls: invalid halfConn.cipher")
	}
	halfConn.Cipher = (*any)(unsafe.Pointer(rawCipher.UnsafeAddr()))

	rawSeq := rawHalfConn.FieldByName("seq")
	if !rawSeq.IsValid() || rawSeq.Kind() != reflect.Array || rawSeq.Len() != 8 || rawSeq.Type().Elem().Kind() != reflect.Uint8 {
		return nil, E.New("badtls: invalid halfConn.seq")
	}
	halfConn.Seq = (*[8]byte)(unsafe.Pointer(rawSeq.UnsafeAddr()))

	rawScratchBuf := rawHalfConn.FieldByName("scratchBuf")
	if !rawScratchBuf.IsValid() || rawScratchBuf.Kind() != reflect.Array || rawScratchBuf.Len() != 13 || rawScratchBuf.Type().Elem().Kind() != reflect.Uint8 {
		return nil, E.New("badtls: invalid halfConn.scratchBuf")
	}
	halfConn.ScratchBuf = (*[13]byte)(unsafe.Pointer(rawScratchBuf.UnsafeAddr()))

	rawTrafficSecret := rawHalfConn.FieldByName("trafficSecret")
	if !rawTrafficSecret.IsValid() || rawTrafficSecret.Kind() != reflect.Slice || rawTrafficSecret.Type().Elem().Kind() != reflect.Uint8 {
		return nil, E.New("badtls: invalid halfConn.trafficSecret")
	}
	halfConn.TrafficSecret = (*[]byte)(unsafe.Pointer(rawTrafficSecret.UnsafeAddr()))

	rawMac := rawHalfConn.FieldByName("mac")
	if !rawMac.IsValid() || rawMac.Kind() != reflect.Interface {
		return nil, E.New("badtls: invalid halfConn.mac")
	}
	halfConn.Mac = (*hash.Hash)(unsafe.Pointer(rawMac.UnsafeAddr()))

	rawKey := rawHalfConn.FieldByName("rawKey")
	if rawKey.IsValid() {
		if /*!rawKey.IsValid() || */ rawKey.Kind() != reflect.Slice || rawKey.Type().Elem().Kind() != reflect.Uint8 {
			return nil, E.New("badtls: invalid halfConn.rawKey")
		}
		halfConn.RawKey = (*[]byte)(unsafe.Pointer(rawKey.UnsafeAddr()))

		rawIV := rawHalfConn.FieldByName("rawIV")
		if !rawIV.IsValid() || rawIV.Kind() != reflect.Slice || rawIV.Type().Elem().Kind() != reflect.Uint8 {
			return nil, E.New("badtls: invalid halfConn.rawIV")
		}
		halfConn.RawIV = (*[]byte)(unsafe.Pointer(rawIV.UnsafeAddr()))

		rawMAC := rawHalfConn.FieldByName("rawMac")
		if !rawMAC.IsValid() || rawMAC.Kind() != reflect.Slice || rawMAC.Type().Elem().Kind() != reflect.Uint8 {
			return nil, E.New("badtls: invalid halfConn.rawMac")
		}
		halfConn.RawMac = (*[]byte)(unsafe.Pointer(rawMAC.UnsafeAddr()))
	}

	return halfConn, nil
}

func (hc *RawHalfConn) Decrypt(record []byte) ([]byte, uint8, error) {
	return hc.methods.decrypt(hc.pointer, record)
}

func (hc *RawHalfConn) SetErrorLocked(err error) error {
	return hc.methods.setErrorLocked(hc.pointer, err)
}

func (hc *RawHalfConn) SetTrafficSecret(suite unsafe.Pointer, level int, secret []byte) {
	hc.methods.setTrafficSecret(hc.pointer, suite, level, secret)
}

func (hc *RawHalfConn) ExplicitNonceLen() int {
	return hc.methods.explicitNonceLen(hc.pointer)
}
