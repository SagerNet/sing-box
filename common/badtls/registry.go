//go:build go1.25 && badlinkname

package badtls

import (
	"crypto/tls"
	"net"
	"unsafe"
)

type Methods struct {
	readRecord                 func(c unsafe.Pointer) error
	handlePostHandshakeMessage func(c unsafe.Pointer) error
	writeRecordLocked          func(c unsafe.Pointer, typ uint16, data []byte) (int, error)

	setErrorLocked   func(hc unsafe.Pointer, err error) error
	decrypt          func(hc unsafe.Pointer, record []byte) ([]byte, uint8, error)
	setTrafficSecret func(hc unsafe.Pointer, suite unsafe.Pointer, level int, secret []byte)
	explicitNonceLen func(hc unsafe.Pointer) int
}

var methodRegistry []func(conn net.Conn) (unsafe.Pointer, *Methods, bool)

func init() {
	methodRegistry = append(methodRegistry, func(conn net.Conn) (unsafe.Pointer, *Methods, bool) {
		tlsConn, loaded := conn.(*tls.Conn)
		if !loaded {
			return nil, nil, false
		}
		return unsafe.Pointer(tlsConn), &Methods{
			readRecord:                 stdTLSReadRecord,
			handlePostHandshakeMessage: stdTLSHandlePostHandshakeMessage,
			writeRecordLocked:          stdWriteRecordLocked,

			setErrorLocked:   stdSetErrorLocked,
			decrypt:          stdDecrypt,
			setTrafficSecret: stdSetTrafficSecret,
			explicitNonceLen: stdExplicitNonceLen,
		}, true
	})
}

//go:linkname stdTLSReadRecord crypto/tls.(*Conn).readRecord
func stdTLSReadRecord(c unsafe.Pointer) error

//go:linkname stdTLSHandlePostHandshakeMessage crypto/tls.(*Conn).handlePostHandshakeMessage
func stdTLSHandlePostHandshakeMessage(c unsafe.Pointer) error

//go:linkname stdWriteRecordLocked crypto/tls.(*Conn).writeRecordLocked
func stdWriteRecordLocked(c unsafe.Pointer, typ uint16, data []byte) (int, error)

//go:linkname stdSetErrorLocked crypto/tls.(*halfConn).setErrorLocked
func stdSetErrorLocked(hc unsafe.Pointer, err error) error

//go:linkname stdDecrypt crypto/tls.(*halfConn).decrypt
func stdDecrypt(hc unsafe.Pointer, record []byte) ([]byte, uint8, error)

//go:linkname stdSetTrafficSecret crypto/tls.(*halfConn).setTrafficSecret
func stdSetTrafficSecret(hc unsafe.Pointer, suite unsafe.Pointer, level int, secret []byte)

//go:linkname stdExplicitNonceLen crypto/tls.(*halfConn).explicitNonceLen
func stdExplicitNonceLen(hc unsafe.Pointer) int
