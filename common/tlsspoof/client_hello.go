package tlsspoof

import (
	"encoding/binary"

	tf "github.com/sagernet/sing-box/common/tlsfragment"
	E "github.com/sagernet/sing/common/exceptions"
)

const (
	recordLengthOffset    = 3
	handshakeLengthOffset = 6
)

// server_name extension layout (RFC 6066 §3). Offsets are relative to the
// SNI host name (index returned by the parser):
//
//	...    uint16 extension_type = 0x0000     (host_name - 9)
//	...    uint16 extension_data_length       (host_name - 7)
//	...    uint16 server_name_list_length     (host_name - 5)
//	...    uint8  name_type = host_name       (host_name - 3)
//	...    uint16 host_name_length            (host_name - 2)
//	sni    host_name                          (host_name)
const (
	extensionDataLengthOffsetFromSNI = -7
	listLengthOffsetFromSNI          = -5
	hostNameLengthOffsetFromSNI      = -2
)

func rewriteSNI(record []byte, fakeSNI string) ([]byte, error) {
	if len(fakeSNI) > 0xFFFF {
		return nil, E.New("fake SNI too long: ", len(fakeSNI), " bytes")
	}
	serverName := tf.IndexTLSServerName(record)
	if serverName == nil {
		return nil, E.New("not a ClientHello with SNI")
	}

	delta := len(fakeSNI) - serverName.Length
	out := make([]byte, len(record)+delta)
	copy(out, record[:serverName.Index])
	copy(out[serverName.Index:], fakeSNI)
	copy(out[serverName.Index+len(fakeSNI):], record[serverName.Index+serverName.Length:])

	err := patchUint16(out, recordLengthOffset, delta)
	if err != nil {
		return nil, E.Cause(err, "patch record length")
	}
	err = patchUint24(out, handshakeLengthOffset, delta)
	if err != nil {
		return nil, E.Cause(err, "patch handshake length")
	}
	for _, off := range []int{
		serverName.ExtensionsListLengthIndex,
		serverName.Index + extensionDataLengthOffsetFromSNI,
		serverName.Index + listLengthOffsetFromSNI,
		serverName.Index + hostNameLengthOffsetFromSNI,
	} {
		err = patchUint16(out, off, delta)
		if err != nil {
			return nil, E.Cause(err, "patch length at offset ", off)
		}
	}
	return out, nil
}

func patchUint16(data []byte, offset, delta int) error {
	patched := int(binary.BigEndian.Uint16(data[offset:])) + delta
	if patched < 0 || patched > 0xFFFF {
		return E.New("uint16 out of range: ", patched)
	}
	binary.BigEndian.PutUint16(data[offset:], uint16(patched))
	return nil
}

func patchUint24(data []byte, offset, delta int) error {
	original := int(data[offset])<<16 | int(data[offset+1])<<8 | int(data[offset+2])
	patched := original + delta
	if patched < 0 || patched > 0xFFFFFF {
		return E.New("uint24 out of range: ", patched)
	}
	data[offset] = byte(patched >> 16)
	data[offset+1] = byte(patched >> 8)
	data[offset+2] = byte(patched)
	return nil
}
