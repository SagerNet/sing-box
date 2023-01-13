package mtproto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"io"

	E "github.com/sagernet/sing/common/exceptions"
)

const (
	handshakeFrameLen = 64

	handshakeFrameLenKey            = 32
	handshakeFrameLenIV             = 16
	handshakeFrameLenConnectionType = 4

	handshakeFrameOffsetStart          = 8
	handshakeFrameOffsetKey            = handshakeFrameOffsetStart
	handshakeFrameOffsetIV             = handshakeFrameOffsetKey + handshakeFrameLenKey
	handshakeFrameOffsetConnectionType = handshakeFrameOffsetIV + handshakeFrameLenIV
	handshakeFrameOffsetDC             = handshakeFrameOffsetConnectionType + handshakeFrameLenConnectionType
)

// Connection-Type: Secure. We support only fake tls.
var handshakeConnectionType = []byte{0xdd, 0xdd, 0xdd, 0xdd}

// A structure of obfuscated2 handshake frame is following:
//
//	[frameOffsetFirst:frameOffsetKey:frameOffsetIV:frameOffsetMagic:frameOffsetDC:frameOffsetEnd].
//
//	- 8 bytes of noise
//	- 32 bytes of AES Key
//	- 16 bytes of AES IV
//	- 4 bytes of 'connection type' - this has some setting like a connection type
//	- 2 bytes of 'DC'. DC is little endian int16
//	- 2 bytes of noise
type handshakeFrame struct {
	data [handshakeFrameLen]byte
}

type clientHandshakeFrame struct {
	handshakeFrame
}

func (f *clientHandshakeFrame) dc() int {
	idx := int16(f.data[handshakeFrameOffsetDC]) | int16(f.data[handshakeFrameOffsetDC+1])<<8 //nolint: gomnd, lll // little endian for int16 is here

	switch {
	case idx > 0:
		return int(idx)
	case idx < 0:
		return -int(idx)
	default:
		return 2
	}
}

func (f *handshakeFrame) key() []byte {
	return f.data[handshakeFrameOffsetKey:handshakeFrameOffsetIV]
}

func (f *handshakeFrame) iv() []byte {
	return f.data[handshakeFrameOffsetIV:handshakeFrameOffsetConnectionType]
}

func (f *handshakeFrame) connectionType() []byte {
	return f.data[handshakeFrameOffsetConnectionType:handshakeFrameOffsetDC]
}

func (f *handshakeFrame) invert() *clientHandshakeFrame {
	copyFrame := &clientHandshakeFrame{}

	for i := 0; i < handshakeFrameLenKey+handshakeFrameLenIV; i++ {
		copyFrame.data[handshakeFrameOffsetKey+i] = f.data[handshakeFrameOffsetConnectionType-1-i]
	}

	return copyFrame
}

func (f *clientHandshakeFrame) decryptor(secret []byte) cipher.Stream {
	hasher := sha256.New()

	hasher.Write(f.key())
	hasher.Write(secret)

	return makeAesCtr(hasher.Sum(nil), f.iv())
}

func (f *clientHandshakeFrame) encryptor(secret []byte) cipher.Stream {
	invertedHandshake := f.invert()

	hasher := sha256.New()

	hasher.Write(invertedHandshake.key())
	hasher.Write(secret)

	return makeAesCtr(hasher.Sum(nil), invertedHandshake.iv())
}

func makeAesCtr(key, iv []byte) cipher.Stream {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	return cipher.NewCTR(block, iv)
}

type serverHandshakeFrame struct {
	handshakeFrame
}

func (s *serverHandshakeFrame) decryptor() cipher.Stream {
	invertedHandshake := s.invert()

	return makeAesCtr(invertedHandshake.key(), invertedHandshake.iv())
}

func (s *serverHandshakeFrame) encryptor() cipher.Stream {
	return makeAesCtr(s.key(), s.iv())
}

func GenerateObfs2ServerHandshake() (cipher.Stream, cipher.Stream, []byte) {
	handshake := generateServerHanshakeFrame()
	copyHandshake := handshake
	encryptor := handshake.encryptor()
	decryptor := handshake.decryptor()

	encryptor.XORKeyStream(handshake.data[:], handshake.data[:])
	copy(handshake.key(), copyHandshake.key())
	copy(handshake.iv(), copyHandshake.iv())

	return encryptor, decryptor, handshake.data[:]
}

func generateServerHanshakeFrame() serverHandshakeFrame {
	frame := serverHandshakeFrame{}

	for {
		if _, err := rand.Read(frame.data[:]); err != nil {
			panic(err)
		}

		if frame.data[0] == 0xEF { //nolint: gomnd // taken from tg sources
			continue
		}

		switch binary.LittleEndian.Uint32(frame.data[:4]) {
		case 0x44414548, 0x54534F50, 0x20544547, 0x4954504F, 0xEEEEEEEE: //nolint: gomnd // taken from tg sources
			continue
		}

		if frame.data[4]|frame.data[5]|frame.data[6]|frame.data[7] == 0 {
			continue
		}

		copy(frame.connectionType(), handshakeConnectionType)

		return frame
	}
}

func Obfs2ClientHandshake(secret []byte, conn *FakeTLSConn) (int, error) {
	handshake := &clientHandshakeFrame{}

	if _, err := io.ReadFull(conn, handshake.data[:]); err != nil {
		return 0, E.Cause(err, "cannot read frame")
	}

	decryptor := handshake.decryptor(secret)
	encryptor := handshake.encryptor(secret)

	decryptor.XORKeyStream(handshake.data[:], handshake.data[:])

	if val := handshake.connectionType(); subtle.ConstantTimeCompare(handshakeConnectionType, val) != 1 {
		return 0, E.New("unsupported connection type: ", hex.EncodeToString(val))
	}

	conn.SetupObfs2(encryptor, decryptor)
	return handshake.dc(), nil
}
