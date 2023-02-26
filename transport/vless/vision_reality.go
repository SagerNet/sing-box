//go:build with_reality_server

package vless

import (
	"net"
	"reflect"
	"unsafe"

	"github.com/sagernet/reality"
	"github.com/sagernet/sing/common"
)

func init() {
	tlsRegistry = append(tlsRegistry, func(conn net.Conn) (loaded bool, netConn net.Conn, reflectType reflect.Type, reflectPointer uintptr) {
		tlsConn, loaded := common.Cast[*reality.Conn](conn)
		if !loaded {
			return
		}
		return true, tlsConn.NetConn(), reflect.TypeOf(tlsConn).Elem(), uintptr(unsafe.Pointer(tlsConn))
	})
}
