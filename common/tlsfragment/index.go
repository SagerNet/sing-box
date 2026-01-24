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

type MyServerName struct {
	Index      int
	Length     int
	ServerName string
}

func IndexTLSServerName(payload []byte) *MyServerName {
	if len(payload) < recordLayerHeaderLen || payload[0] != contentType {
		return nil
	}
	segmentLen := binary.BigEndian.Uint16(payload[3:5])
	if len(payload) < recordLayerHeaderLen+int(segmentLen) {
		return nil
	}
	serverName := indexTLSServerNameFromHandshake(payload[recordLayerHeaderLen:])
	if serverName == nil {
		return nil
	}
	serverName.Index += recordLayerHeaderLen
	return serverName
}

func indexTLSServerNameFromHandshake(handshake []byte) *MyServerName {
	if len(handshake) < handshakeHeaderLen+randomDataLen+sessionIDHeaderLen {
		return nil
	}
	if handshake[0] != handshakeType {
		return nil
	}
	handshakeLen := uint32(handshake[1])<<16 | uint32(handshake[2])<<8 | uint32(handshake[3])
	if len(handshake[4:]) != int(handshakeLen) {
		return nil
	}
	tlsVersion := uint16(handshake[4])<<8 | uint16(handshake[5])
	if tlsVersion&tlsVersionBitmask != 0x0300 && tlsVersion != tls13 {
		return nil
	}
	sessionIDLen := handshake[38]
	currentIndex := handshakeHeaderLen + randomDataLen + sessionIDHeaderLen + int(sessionIDLen)
	if len(handshake) < currentIndex {
		return nil
	}
	cipherSuites := handshake[currentIndex:]
	if len(cipherSuites) < cipherSuiteHeaderLen {
		return nil
	}
	csLen := uint16(cipherSuites[0])<<8 | uint16(cipherSuites[1])
	if len(cipherSuites) < cipherSuiteHeaderLen+int(csLen)+compressMethodHeaderLen {
		return nil
	}
	compressMethodLen := uint16(cipherSuites[cipherSuiteHeaderLen+int(csLen)])
	currentIndex += cipherSuiteHeaderLen + int(csLen) + compressMethodHeaderLen + int(compressMethodLen)
	if len(handshake) < currentIndex {
		return nil
	}
	serverName := indexTLSServerNameFromExtensions(handshake[currentIndex:])
	if serverName == nil {
		return nil
	}
	serverName.Index += currentIndex
	return serverName
}

func indexTLSServerNameFromExtensions(exs []byte) *MyServerName {
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

			return &MyServerName{
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
