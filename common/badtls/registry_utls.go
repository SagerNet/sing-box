//go:build go1.25 && badlinkname

package badtls

import (
	"net"
	"unsafe"

	N "github.com/sagernet/sing/common/network"

	"github.com/metacubex/utls"
)

func init() {
	methodRegistry = append(methodRegistry, func(conn net.Conn) (unsafe.Pointer, *Methods, bool) {
		var pointer unsafe.Pointer
		if uConn, loaded := N.CastReader[*tls.Conn](conn); loaded {
			pointer = unsafe.Pointer(uConn)
		} else if uConn, loaded := N.CastReader[*tls.UConn](conn); loaded {
			pointer = unsafe.Pointer(uConn.Conn)
		} else {
			return nil, nil, false
		}
		return pointer, &Methods{
			readRecord:                 utlsReadRecord,
			handlePostHandshakeMessage: utlsHandlePostHandshakeMessage,
			writeRecordLocked:          utlsWriteRecordLocked,

			setErrorLocked:   utlsSetErrorLocked,
			decrypt:          utlsDecrypt,
			setTrafficSecret: utlsSetTrafficSecret,
			explicitNonceLen: utlsExplicitNonceLen,
		}, true
	})
}

//go:linkname utlsReadRecord github.com/metacubex/utls.(*Conn).readRecord
func utlsReadRecord(c unsafe.Pointer) error

//go:linkname utlsHandlePostHandshakeMessage github.com/metacubex/utls.(*Conn).handlePostHandshakeMessage
func utlsHandlePostHandshakeMessage(c unsafe.Pointer) error

//go:linkname utlsWriteRecordLocked github.com/metacubex/utls.(*Conn).writeRecordLocked
func utlsWriteRecordLocked(hc unsafe.Pointer, typ uint16, data []byte) (int, error)

//go:linkname utlsSetErrorLocked github.com/metacubex/utls.(*halfConn).setErrorLocked
func utlsSetErrorLocked(hc unsafe.Pointer, err error) error

//go:linkname utlsDecrypt github.com/metacubex/utls.(*halfConn).decrypt
func utlsDecrypt(hc unsafe.Pointer, record []byte) ([]byte, uint8, error)

//go:linkname utlsSetTrafficSecret github.com/metacubex/utls.(*halfConn).setTrafficSecret
func utlsSetTrafficSecret(hc unsafe.Pointer, suite unsafe.Pointer, level int, secret []byte)

//go:linkname utlsExplicitNonceLen github.com/metacubex/utls.(*halfConn).explicitNonceLen
func utlsExplicitNonceLen(hc unsafe.Pointer) int
