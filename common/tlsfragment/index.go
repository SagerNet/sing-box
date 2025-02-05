package tf

import (
	"encoding/binary"
)

const (
	recordLayerHeaderLen    int    = 5
	handshakeHeaderLen      int    = 6
	randomDataLen           int    = 32
	sessionIDHeaderLen      int    = 1
	cipherSuiteHeaderLen    int    = 2
	compressMethodHeaderLen int    = 1
	extensionsHeaderLen     int    = 2
	extensionHeaderLen      int    = 4
	sniExtensionHeaderLen   int    = 5
	contentType             uint8  = 22
	handshakeType           uint8  = 1
	sniExtensionType        uint16 = 0
	sniNameDNSHostnameType  uint8  = 0
	tlsVersionBitmask       uint16 = 0xFFFC
	tls13                   uint16 = 0x0304
)

type myServerName struct {
	Index      int
	Length     int
	ServerName string
}

func indexTLSServerName(payload []byte) *myServerName {
	if len(payload) < recordLayerHeaderLen || payload[0] != contentType {
		return nil
	}
	segmentLen := binary.BigEndian.Uint16(payload[3:5])
	if len(payload) < recordLayerHeaderLen+int(segmentLen) {
		return nil
	}
	serverName := indexTLSServerNameFromHandshake(payload[recordLayerHeaderLen : recordLayerHeaderLen+int(segmentLen)])
	if serverName == nil {
		return nil
	}
	serverName.Length += recordLayerHeaderLen
	return serverName
}

func indexTLSServerNameFromHandshake(hs []byte) *myServerName {
	if len(hs) < handshakeHeaderLen+randomDataLen+sessionIDHeaderLen {
		return nil
	}
	if hs[0] != handshakeType {
		return nil
	}
	handshakeLen := uint32(hs[1])<<16 | uint32(hs[2])<<8 | uint32(hs[3])
	if len(hs[4:]) != int(handshakeLen) {
		return nil
	}
	tlsVersion := uint16(hs[4])<<8 | uint16(hs[5])
	if tlsVersion&tlsVersionBitmask != 0x0300 && tlsVersion != tls13 {
		return nil
	}
	sessionIDLen := hs[38]
	if len(hs) < handshakeHeaderLen+randomDataLen+sessionIDHeaderLen+int(sessionIDLen) {
		return nil
	}
	cs := hs[handshakeHeaderLen+randomDataLen+sessionIDHeaderLen+int(sessionIDLen):]
	if len(cs) < cipherSuiteHeaderLen {
		return nil
	}
	csLen := uint16(cs[0])<<8 | uint16(cs[1])
	if len(cs) < cipherSuiteHeaderLen+int(csLen)+compressMethodHeaderLen {
		return nil
	}
	compressMethodLen := uint16(cs[cipherSuiteHeaderLen+int(csLen)])
	if len(cs) < cipherSuiteHeaderLen+int(csLen)+compressMethodHeaderLen+int(compressMethodLen) {
		return nil
	}
	currentIndex := cipherSuiteHeaderLen + int(csLen) + compressMethodHeaderLen + int(compressMethodLen)
	serverName := indexTLSServerNameFromExtensions(cs[currentIndex:])
	if serverName == nil {
		return nil
	}
	serverName.Index += currentIndex
	return serverName
}

func indexTLSServerNameFromExtensions(exs []byte) *myServerName {
	if len(exs) == 0 {
		return nil
	}
	if len(exs) < extensionsHeaderLen {
		return nil
	}
	exsLen := uint16(exs[0])<<8 | uint16(exs[1])
	exs = exs[extensionsHeaderLen:]
	if len(exs) < int(exsLen) {
		return nil
	}
	for currentIndex := extensionsHeaderLen; len(exs) > 0; {
		if len(exs) < extensionHeaderLen {
			return nil
		}
		exType := uint16(exs[0])<<8 | uint16(exs[1])
		exLen := uint16(exs[2])<<8 | uint16(exs[3])
		if len(exs) < extensionHeaderLen+int(exLen) {
			return nil
		}
		sex := exs[extensionHeaderLen : extensionHeaderLen+int(exLen)]

		switch exType {
		case sniExtensionType:
			if len(sex) < sniExtensionHeaderLen {
				return nil
			}
			sniType := sex[2]
			if sniType != sniNameDNSHostnameType {
				return nil
			}
			sniLen := uint16(sex[3])<<8 | uint16(sex[4])
			sex = sex[sniExtensionHeaderLen:]
			return &myServerName{
				Index:      currentIndex + extensionHeaderLen + sniExtensionHeaderLen,
				Length:     int(sniLen),
				ServerName: string(sex),
			}
		}
		exs = exs[4+exLen:]
		currentIndex += 4 + int(exLen)
	}
	return nil
}
