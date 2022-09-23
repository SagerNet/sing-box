package v2raywebsocket

import _ "unsafe"

//go:linkname maskBytes github.com/sagernet/websocket.maskBytes
func maskBytes(key [4]byte, pos int, b []byte) int
