// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && go1.25 && badlinkname

package ktls

import (
	"fmt"

	"golang.org/x/crypto/cryptobyte"
)

// The marshalingFunction type is an adapter to allow the use of ordinary
// functions as cryptobyte.MarshalingValue.
type marshalingFunction func(b *cryptobyte.Builder) error

func (f marshalingFunction) Marshal(b *cryptobyte.Builder) error {
	return f(b)
}

// addBytesWithLength appends a sequence of bytes to the cryptobyte.Builder. If
// the length of the sequence is not the value specified, it produces an error.
func addBytesWithLength(b *cryptobyte.Builder, v []byte, n int) {
	b.AddValue(marshalingFunction(func(b *cryptobyte.Builder) error {
		if len(v) != n {
			return fmt.Errorf("invalid value length: expected %d, got %d", n, len(v))
		}
		b.AddBytes(v)
		return nil
	}))
}

// addUint64 appends a big-endian, 64-bit value to the cryptobyte.Builder.
func addUint64(b *cryptobyte.Builder, v uint64) {
	b.AddUint32(uint32(v >> 32))
	b.AddUint32(uint32(v))
}

// readUint64 decodes a big-endian, 64-bit value into out and advances over it.
// It reports whether the read was successful.
func readUint64(s *cryptobyte.String, out *uint64) bool {
	var hi, lo uint32
	if !s.ReadUint32(&hi) || !s.ReadUint32(&lo) {
		return false
	}
	*out = uint64(hi)<<32 | uint64(lo)
	return true
}

// readUint8LengthPrefixed acts like s.ReadUint8LengthPrefixed, but targets a
// []byte instead of a cryptobyte.String.
func readUint8LengthPrefixed(s *cryptobyte.String, out *[]byte) bool {
	return s.ReadUint8LengthPrefixed((*cryptobyte.String)(out))
}

// readUint16LengthPrefixed acts like s.ReadUint16LengthPrefixed, but targets a
// []byte instead of a cryptobyte.String.
func readUint16LengthPrefixed(s *cryptobyte.String, out *[]byte) bool {
	return s.ReadUint16LengthPrefixed((*cryptobyte.String)(out))
}

// readUint24LengthPrefixed acts like s.ReadUint24LengthPrefixed, but targets a
// []byte instead of a cryptobyte.String.
func readUint24LengthPrefixed(s *cryptobyte.String, out *[]byte) bool {
	return s.ReadUint24LengthPrefixed((*cryptobyte.String)(out))
}

type keyUpdateMsg struct {
	updateRequested bool
}

func (m *keyUpdateMsg) marshal() ([]byte, error) {
	var b cryptobyte.Builder
	b.AddUint8(typeKeyUpdate)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		if m.updateRequested {
			b.AddUint8(1)
		} else {
			b.AddUint8(0)
		}
	})

	return b.Bytes()
}

func (m *keyUpdateMsg) unmarshal(data []byte) bool {
	s := cryptobyte.String(data)

	var updateRequested uint8
	if !s.Skip(4) || // message type and uint24 length field
		!s.ReadUint8(&updateRequested) || !s.Empty() {
		return false
	}
	switch updateRequested {
	case 0:
		m.updateRequested = false
	case 1:
		m.updateRequested = true
	default:
		return false
	}
	return true
}

// TLS handshake message types.
const (
	typeHelloRequest          uint8 = 0
	typeClientHello           uint8 = 1
	typeServerHello           uint8 = 2
	typeNewSessionTicket      uint8 = 4
	typeEndOfEarlyData        uint8 = 5
	typeEncryptedExtensions   uint8 = 8
	typeCertificate           uint8 = 11
	typeServerKeyExchange     uint8 = 12
	typeCertificateRequest    uint8 = 13
	typeServerHelloDone       uint8 = 14
	typeCertificateVerify     uint8 = 15
	typeClientKeyExchange     uint8 = 16
	typeFinished              uint8 = 20
	typeCertificateStatus     uint8 = 22
	typeKeyUpdate             uint8 = 24
	typeCompressedCertificate uint8 = 25
	typeMessageHash           uint8 = 254 // synthetic message
)

// TLS compression types.
const (
	compressionNone uint8 = 0
)

// TLS extension numbers
const (
	extensionServerName              uint16 = 0
	extensionStatusRequest           uint16 = 5
	extensionSupportedCurves         uint16 = 10 // supported_groups in TLS 1.3, see RFC 8446, Section 4.2.7
	extensionSupportedPoints         uint16 = 11
	extensionSignatureAlgorithms     uint16 = 13
	extensionALPN                    uint16 = 16
	extensionSCT                     uint16 = 18
	extensionPadding                 uint16 = 21
	extensionExtendedMasterSecret    uint16 = 23
	extensionCompressCertificate     uint16 = 27 // compress_certificate in TLS 1.3
	extensionSessionTicket           uint16 = 35
	extensionPreSharedKey            uint16 = 41
	extensionEarlyData               uint16 = 42
	extensionSupportedVersions       uint16 = 43
	extensionCookie                  uint16 = 44
	extensionPSKModes                uint16 = 45
	extensionCertificateAuthorities  uint16 = 47
	extensionSignatureAlgorithmsCert uint16 = 50
	extensionKeyShare                uint16 = 51
	extensionQUICTransportParameters uint16 = 57
	extensionALPS                    uint16 = 17513
	extensionRenegotiationInfo       uint16 = 0xff01
	extensionECHOuterExtensions      uint16 = 0xfd00
	extensionEncryptedClientHello    uint16 = 0xfe0d
)

type handshakeMessage interface {
	marshal() ([]byte, error)
	unmarshal([]byte) bool
}
type newSessionTicketMsgTLS13 struct {
	lifetime     uint32
	ageAdd       uint32
	nonce        []byte
	label        []byte
	maxEarlyData uint32
}

func (m *newSessionTicketMsgTLS13) marshal() ([]byte, error) {
	var b cryptobyte.Builder
	b.AddUint8(typeNewSessionTicket)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddUint32(m.lifetime)
		b.AddUint32(m.ageAdd)
		b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes(m.nonce)
		})
		b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes(m.label)
		})

		b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			if m.maxEarlyData > 0 {
				b.AddUint16(extensionEarlyData)
				b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
					b.AddUint32(m.maxEarlyData)
				})
			}
		})
	})

	return b.Bytes()
}

func (m *newSessionTicketMsgTLS13) unmarshal(data []byte) bool {
	*m = newSessionTicketMsgTLS13{}
	s := cryptobyte.String(data)

	var extensions cryptobyte.String
	if !s.Skip(4) || // message type and uint24 length field
		!s.ReadUint32(&m.lifetime) ||
		!s.ReadUint32(&m.ageAdd) ||
		!readUint8LengthPrefixed(&s, &m.nonce) ||
		!readUint16LengthPrefixed(&s, &m.label) ||
		!s.ReadUint16LengthPrefixed(&extensions) ||
		!s.Empty() {
		return false
	}

	for !extensions.Empty() {
		var extension uint16
		var extData cryptobyte.String
		if !extensions.ReadUint16(&extension) ||
			!extensions.ReadUint16LengthPrefixed(&extData) {
			return false
		}

		switch extension {
		case extensionEarlyData:
			if !extData.ReadUint32(&m.maxEarlyData) {
				return false
			}
		default:
			// Ignore unknown extensions.
			continue
		}

		if !extData.Empty() {
			return false
		}
	}

	return true
}
