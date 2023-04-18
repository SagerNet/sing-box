package vless

import (
	"bytes"

	"github.com/sagernet/sing/common/buf"
)

var (
	tls13SupportedVersions  = []byte{0x00, 0x2b, 0x00, 0x02, 0x03, 0x04}
	tlsClientHandShakeStart = []byte{0x16, 0x03}
	tlsServerHandShakeStart = []byte{0x16, 0x03, 0x03}
	tlsApplicationDataStart = []byte{0x17, 0x03, 0x03}
)

const (
	commandPaddingContinue byte = iota
	commandPaddingEnd
	commandPaddingDirect
)

var tls13CipherSuiteDic = map[uint16]string{
	0x1301: "TLS_AES_128_GCM_SHA256",
	0x1302: "TLS_AES_256_GCM_SHA384",
	0x1303: "TLS_CHACHA20_POLY1305_SHA256",
	0x1304: "TLS_AES_128_CCM_SHA256",
	0x1305: "TLS_AES_128_CCM_8_SHA256",
}

func reshapeBuffer(b []byte) []*buf.Buffer {
	const bufferLimit = 8192 - 21
	if len(b) < bufferLimit {
		return []*buf.Buffer{buf.As(b)}
	}
	index := int32(bytes.LastIndex(b, tlsApplicationDataStart))
	if index <= 0 {
		index = 8192 / 2
	}
	return []*buf.Buffer{buf.As(b[:index]), buf.As(b[index:])}
}
