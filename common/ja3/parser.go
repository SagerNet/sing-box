// Copyright (c) 2018, Open Systems AG. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package ja3

import (
	"encoding/binary"
	"strconv"
)

const (
	// Constants used for parsing
	recordLayerHeaderLen                  int    = 5
	handshakeHeaderLen                    int    = 6
	randomDataLen                         int    = 32
	sessionIDHeaderLen                    int    = 1
	cipherSuiteHeaderLen                  int    = 2
	compressMethodHeaderLen               int    = 1
	extensionsHeaderLen                   int    = 2
	extensionHeaderLen                    int    = 4
	sniExtensionHeaderLen                 int    = 5
	ecExtensionHeaderLen                  int    = 2
	ecpfExtensionHeaderLen                int    = 1
	versionExtensionHeaderLen             int    = 1
	signatureAlgorithmsExtensionHeaderLen int    = 2
	contentType                           uint8  = 22
	handshakeType                         uint8  = 1
	sniExtensionType                      uint16 = 0
	sniNameDNSHostnameType                uint8  = 0
	ecExtensionType                       uint16 = 10
	ecpfExtensionType                     uint16 = 11
	versionExtensionType                  uint16 = 43
	signatureAlgorithmsExtensionType      uint16 = 13

	// Versions
	// The bitmask covers the versions SSL3.0 to TLS1.2
	tlsVersionBitmask uint16 = 0xFFFC
	tls13             uint16 = 0x0304

	// GREASE values
	// The bitmask covers all GREASE values
	GreaseBitmask uint16 = 0x0F0F

	// Constants used for marshalling
	dashByte  = byte(45)
	commaByte = byte(44)
)

// parseSegment to populate the corresponding ClientHello object or return an error
func (j *ClientHello) parseSegment(segment []byte) error {
	// Check if we can decode the next fields
	if len(segment) < recordLayerHeaderLen {
		return &ParseError{LengthErr, 1}
	}

	// Check if we have "Content Type: Handshake (22)"
	contType := uint8(segment[0])
	if contType != contentType {
		return &ParseError{errType: ContentTypeErr}
	}

	// Check if TLS record layer version is supported
	tlsRecordVersion := uint16(segment[1])<<8 | uint16(segment[2])
	if tlsRecordVersion&tlsVersionBitmask != 0x0300 && tlsRecordVersion != tls13 {
		return &ParseError{VersionErr, 1}
	}

	// Check that the Handshake is as long as expected from the length field
	segmentLen := uint16(segment[3])<<8 | uint16(segment[4])
	if len(segment[recordLayerHeaderLen:]) < int(segmentLen) {
		return &ParseError{LengthErr, 2}
	}
	// Keep the Handshake messege, ignore any additional following record types
	hs := segment[recordLayerHeaderLen : recordLayerHeaderLen+int(segmentLen)]

	err := j.parseHandshake(hs)

	return err
}

// parseHandshake body
func (j *ClientHello) parseHandshake(hs []byte) error {
	// Check if we can decode the next fields
	if len(hs) < handshakeHeaderLen+randomDataLen+sessionIDHeaderLen {
		return &ParseError{LengthErr, 3}
	}

	// Check if we have "Handshake Type: Client Hello (1)"
	handshType := uint8(hs[0])
	if handshType != handshakeType {
		return &ParseError{errType: HandshakeTypeErr}
	}

	// Check if actual length of handshake matches (this is a great exclusion criterion for false positives,
	// as these fields have to match the actual length of the rest of the segment)
	handshakeLen := uint32(hs[1])<<16 | uint32(hs[2])<<8 | uint32(hs[3])
	if len(hs[4:]) != int(handshakeLen) {
		return &ParseError{LengthErr, 4}
	}

	// Check if Client Hello version is supported
	tlsVersion := uint16(hs[4])<<8 | uint16(hs[5])
	if tlsVersion&tlsVersionBitmask != 0x0300 && tlsVersion != tls13 {
		return &ParseError{VersionErr, 2}
	}
	j.Version = tlsVersion

	// Check if we can decode the next fields
	sessionIDLen := uint8(hs[38])
	if len(hs) < handshakeHeaderLen+randomDataLen+sessionIDHeaderLen+int(sessionIDLen) {
		return &ParseError{LengthErr, 5}
	}

	// Cipher Suites
	cs := hs[handshakeHeaderLen+randomDataLen+sessionIDHeaderLen+int(sessionIDLen):]

	// Check if we can decode the next fields
	if len(cs) < cipherSuiteHeaderLen {
		return &ParseError{LengthErr, 6}
	}

	csLen := uint16(cs[0])<<8 | uint16(cs[1])
	numCiphers := int(csLen / 2)
	cipherSuites := make([]uint16, 0, numCiphers)

	// Check if we can decode the next fields
	if len(cs) < cipherSuiteHeaderLen+int(csLen)+compressMethodHeaderLen {
		return &ParseError{LengthErr, 7}
	}

	for i := 0; i < numCiphers; i++ {
		cipherSuite := uint16(cs[2+i<<1])<<8 | uint16(cs[3+i<<1])
		cipherSuites = append(cipherSuites, cipherSuite)
	}
	j.CipherSuites = cipherSuites

	// Check if we can decode the next fields
	compressMethodLen := uint16(cs[cipherSuiteHeaderLen+int(csLen)])
	if len(cs) < cipherSuiteHeaderLen+int(csLen)+compressMethodHeaderLen+int(compressMethodLen) {
		return &ParseError{LengthErr, 8}
	}

	// Extensions
	exs := cs[cipherSuiteHeaderLen+int(csLen)+compressMethodHeaderLen+int(compressMethodLen):]

	err := j.parseExtensions(exs)

	return err
}

// parseExtensions of the handshake
func (j *ClientHello) parseExtensions(exs []byte) error {
	// Check for no extensions, this fields header is nonexistent if no body is used
	if len(exs) == 0 {
		return nil
	}

	// Check if we can decode the next fields
	if len(exs) < extensionsHeaderLen {
		return &ParseError{LengthErr, 9}
	}

	exsLen := uint16(exs[0])<<8 | uint16(exs[1])
	exs = exs[extensionsHeaderLen:]

	// Check if we can decode the next fields
	if len(exs) < int(exsLen) {
		return &ParseError{LengthErr, 10}
	}

	var sni []byte
	var extensions, ellipticCurves []uint16
	var ellipticCurvePF []uint8
	var versions []uint16
	var signatureAlgorithms []uint16
	for len(exs) > 0 {

		// Check if we can decode the next fields
		if len(exs) < extensionHeaderLen {
			return &ParseError{LengthErr, 11}
		}

		exType := uint16(exs[0])<<8 | uint16(exs[1])
		exLen := uint16(exs[2])<<8 | uint16(exs[3])
		// Ignore any GREASE extensions
		extensions = append(extensions, exType)
		// Check if we can decode the next fields
		if len(exs) < extensionHeaderLen+int(exLen) {
			return &ParseError{LengthErr, 12}
		}

		sex := exs[extensionHeaderLen : extensionHeaderLen+int(exLen)]

		switch exType {
		case sniExtensionType: // Extensions: server_name

			// Check if we can decode the next fields
			if len(sex) < sniExtensionHeaderLen {
				return &ParseError{LengthErr, 13}
			}

			sniType := uint8(sex[2])
			sniLen := uint16(sex[3])<<8 | uint16(sex[4])
			sex = sex[sniExtensionHeaderLen:]

			// Check if we can decode the next fields
			if len(sex) != int(sniLen) {
				return &ParseError{LengthErr, 14}
			}

			switch sniType {
			case sniNameDNSHostnameType:
				sni = sex
			default:
				return &ParseError{errType: SNITypeErr}
			}
		case ecExtensionType: // Extensions: supported_groups

			// Check if we can decode the next fields
			if len(sex) < ecExtensionHeaderLen {
				return &ParseError{LengthErr, 15}
			}

			ecsLen := uint16(sex[0])<<8 | uint16(sex[1])
			numCurves := int(ecsLen / 2)
			ellipticCurves = make([]uint16, 0, numCurves)
			sex = sex[ecExtensionHeaderLen:]

			// Check if we can decode the next fields
			if len(sex) != int(ecsLen) {
				return &ParseError{LengthErr, 16}
			}

			for i := 0; i < numCurves; i++ {
				ecType := uint16(sex[i*2])<<8 | uint16(sex[1+i*2])
				ellipticCurves = append(ellipticCurves, ecType)
			}

		case ecpfExtensionType: // Extensions: ec_point_formats

			// Check if we can decode the next fields
			if len(sex) < ecpfExtensionHeaderLen {
				return &ParseError{LengthErr, 17}
			}

			ecpfsLen := uint8(sex[0])
			numPF := int(ecpfsLen)
			ellipticCurvePF = make([]uint8, numPF)
			sex = sex[ecpfExtensionHeaderLen:]

			// Check if we can decode the next fields
			if len(sex) != numPF {
				return &ParseError{LengthErr, 18}
			}

			for i := 0; i < numPF; i++ {
				ellipticCurvePF[i] = uint8(sex[i])
			}
		case versionExtensionType:
			if len(sex) < versionExtensionHeaderLen {
				return &ParseError{LengthErr, 19}
			}
			versionsLen := int(sex[0])
			for i := 0; i < versionsLen; i += 2 {
				versions = append(versions, binary.BigEndian.Uint16(sex[1:][i:]))
			}
		case signatureAlgorithmsExtensionType:
			if len(sex) < signatureAlgorithmsExtensionHeaderLen {
				return &ParseError{LengthErr, 20}
			}
			ssaLen := binary.BigEndian.Uint16(sex)
			for i := 0; i < int(ssaLen); i += 2 {
				signatureAlgorithms = append(signatureAlgorithms, binary.BigEndian.Uint16(sex[2:][i:]))
			}
		}
		exs = exs[4+exLen:]
	}
	j.ServerName = string(sni)
	j.Extensions = extensions
	j.EllipticCurves = ellipticCurves
	j.EllipticCurvePF = ellipticCurvePF
	j.Versions = versions
	j.SignatureAlgorithms = signatureAlgorithms
	return nil
}

// marshalJA3 into a byte string
func (j *ClientHello) marshalJA3() {
	// An uint16 can contain numbers with up to 5 digits and an uint8 can contain numbers with up to 3 digits, but we
	// also need a byte for each separating character, except at the end.
	byteStringLen := 6*(1+len(j.CipherSuites)+len(j.Extensions)+len(j.EllipticCurves)) + 4*len(j.EllipticCurvePF) - 1
	byteString := make([]byte, 0, byteStringLen)

	// Version
	byteString = strconv.AppendUint(byteString, uint64(j.Version), 10)
	byteString = append(byteString, commaByte)

	// Cipher Suites
	if len(j.CipherSuites) != 0 {
		for _, val := range j.CipherSuites {
			if val&GreaseBitmask != 0x0A0A {
				continue
			}
			byteString = strconv.AppendUint(byteString, uint64(val), 10)
			byteString = append(byteString, dashByte)
		}
		// Replace last dash with a comma
		byteString[len(byteString)-1] = commaByte
	} else {
		byteString = append(byteString, commaByte)
	}

	// Extensions
	if len(j.Extensions) != 0 {
		for _, val := range j.Extensions {
			if val&GreaseBitmask != 0x0A0A {
				continue
			}
			byteString = strconv.AppendUint(byteString, uint64(val), 10)
			byteString = append(byteString, dashByte)
		}
		// Replace last dash with a comma
		byteString[len(byteString)-1] = commaByte
	} else {
		byteString = append(byteString, commaByte)
	}

	// Elliptic curves
	if len(j.EllipticCurves) != 0 {
		for _, val := range j.EllipticCurves {
			if val&GreaseBitmask != 0x0A0A {
				continue
			}
			byteString = strconv.AppendUint(byteString, uint64(val), 10)
			byteString = append(byteString, dashByte)
		}
		// Replace last dash with a comma
		byteString[len(byteString)-1] = commaByte
	} else {
		byteString = append(byteString, commaByte)
	}

	// ECPF
	if len(j.EllipticCurvePF) != 0 {
		for _, val := range j.EllipticCurvePF {
			byteString = strconv.AppendUint(byteString, uint64(val), 10)
			byteString = append(byteString, dashByte)
		}
		// Remove last dash
		byteString = byteString[:len(byteString)-1]
	}

	j.ja3ByteString = byteString
}
