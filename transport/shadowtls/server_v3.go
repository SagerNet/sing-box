package shadowtls

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"hash"
	"io"
	"net"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
)

func ExtractFrame(conn net.Conn) (*buf.Buffer, error) {
	var tlsHeader [tlsHeaderSize]byte
	_, err := io.ReadFull(conn, tlsHeader[:])
	if err != nil {
		return nil, err
	}
	length := int(binary.BigEndian.Uint16(tlsHeader[3:]))
	buffer := buf.NewSize(tlsHeaderSize + length)
	common.Must1(buffer.Write(tlsHeader[:]))
	_, err = buffer.ReadFullFrom(conn, length)
	if err != nil {
		buffer.Release()
	}
	return buffer, err
}

func VerifyClientHello(frame []byte, password string) error {
	const minLen = tlsHeaderSize + 1 + 3 + 2 + tlsRandomSize + 1 + tlsSessionIDSize
	const hmacIndex = sessionIDLengthIndex + 1 + tlsSessionIDSize - hmacSize
	if len(frame) < minLen {
		return io.ErrUnexpectedEOF
	} else if frame[0] != handshake {
		return E.New("unexpected record type")
	} else if frame[5] != clientHello {
		return E.New("unexpected handshake type")
	} else if frame[sessionIDLengthIndex] != tlsSessionIDSize {
		return E.New("unexpected session id length")
	}
	hmacSHA1Hash := hmac.New(sha1.New, []byte(password))
	hmacSHA1Hash.Write(frame[tlsHeaderSize:hmacIndex])
	hmacSHA1Hash.Write(rw.ZeroBytes[:4])
	hmacSHA1Hash.Write(frame[hmacIndex+hmacSize:])
	if !hmac.Equal(frame[hmacIndex:hmacIndex+hmacSize], hmacSHA1Hash.Sum(nil)[:hmacSize]) {
		return E.New("hmac mismatch")
	}
	return nil
}

func ExtractServerRandom(frame []byte) []byte {
	const minLen = tlsHeaderSize + 1 + 3 + 2 + tlsRandomSize

	if len(frame) < minLen || frame[0] != handshake || frame[5] != serverHello {
		return nil
	}

	serverRandom := make([]byte, tlsRandomSize)
	copy(serverRandom, frame[serverRandomIndex:serverRandomIndex+tlsRandomSize])
	return serverRandom
}

func IsServerHelloSupportTLS13(frame []byte) bool {
	if len(frame) < sessionIDLengthIndex {
		return false
	}

	reader := bytes.NewReader(frame[sessionIDLengthIndex:])

	var sessionIdLength uint8
	err := binary.Read(reader, binary.BigEndian, &sessionIdLength)
	if err != nil {
		return false
	}
	_, err = io.CopyN(io.Discard, reader, int64(sessionIdLength))
	if err != nil {
		return false
	}

	_, err = io.CopyN(io.Discard, reader, 3)
	if err != nil {
		return false
	}

	var extensionListLength uint16
	err = binary.Read(reader, binary.BigEndian, &extensionListLength)
	if err != nil {
		return false
	}
	for i := uint16(0); i < extensionListLength; i++ {
		var extensionType uint16
		err = binary.Read(reader, binary.BigEndian, &extensionType)
		if err != nil {
			return false
		}
		var extensionLength uint16
		err = binary.Read(reader, binary.BigEndian, &extensionLength)
		if err != nil {
			return false
		}
		if extensionType != 43 {
			_, err = io.CopyN(io.Discard, reader, int64(extensionLength))
			if err != nil {
				return false
			}
			continue
		}
		if extensionLength != 2 {
			return false
		}
		var extensionValue uint16
		err = binary.Read(reader, binary.BigEndian, &extensionValue)
		if err != nil {
			return false
		}
		return extensionValue == 0x0304
	}
	return false
}

func CopyByFrameUntilHMACMatches(conn net.Conn, handshakeConn net.Conn, hmacVerify hash.Hash, hmacReset func()) (*buf.Buffer, error) {
	for {
		frameBuffer, err := ExtractFrame(conn)
		if err != nil {
			return nil, E.Cause(err, "read client record")
		}
		frame := frameBuffer.Bytes()
		if len(frame) > tlsHmacHeaderSize && frame[0] == applicationData {
			hmacReset()
			hmacVerify.Write(frame[tlsHmacHeaderSize:])
			hmacHash := hmacVerify.Sum(nil)[:4]
			if bytes.Equal(hmacHash, frame[tlsHeaderSize:tlsHmacHeaderSize]) {
				hmacReset()
				hmacVerify.Write(frame[tlsHmacHeaderSize:])
				hmacVerify.Write(frame[tlsHeaderSize:tlsHmacHeaderSize])
				frameBuffer.Advance(tlsHmacHeaderSize)
				return frameBuffer, nil
			}
		}
		_, err = handshakeConn.Write(frame)
		frameBuffer.Release()
		if err != nil {
			return nil, E.Cause(err, "write clint frame")
		}
	}
}

func CopyByFrameWithModification(conn net.Conn, handshakeConn net.Conn, password string, serverRandom []byte, hmacWrite hash.Hash) error {
	writeKey := kdf(password, serverRandom)
	writer := bufio.NewVectorisedWriter(handshakeConn)
	for {
		frameBuffer, err := ExtractFrame(conn)
		if err != nil {
			return E.Cause(err, "read server record")
		}
		frame := frameBuffer.Bytes()
		if frame[0] == applicationData {
			xorSlice(frame[tlsHeaderSize:], writeKey)
			hmacWrite.Write(frame[tlsHeaderSize:])
			binary.BigEndian.PutUint16(frame[3:], uint16(len(frame)-tlsHeaderSize+hmacSize))
			hmacHash := hmacWrite.Sum(nil)[:4]
			_, err = bufio.WriteVectorised(writer, [][]byte{frame[:tlsHeaderSize], hmacHash, frame[tlsHeaderSize:]})
			frameBuffer.Release()
			if err != nil {
				return E.Cause(err, "write modified server frame")
			}
		} else {
			_, err = handshakeConn.Write(frame)
			frameBuffer.Release()
			if err != nil {
				return E.Cause(err, "write server frame")
			}
		}
	}
}
