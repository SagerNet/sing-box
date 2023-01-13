package mtproto

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	mrand "math/rand"
	"net"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/replay"

	"golang.org/x/crypto/curve25519"
)

const (
	// TypeChangeCipherSpec defines a byte value of the TLS record when a
	// peer wants to change a specifications of the chosen cipher.
	TypeChangeCipherSpec byte = 0x14

	// TypeHandshake defines a byte value of the TLS record when a peer
	// initiates a new TLS connection and wants to make a handshake
	// ceremony.
	TypeHandshake byte = 0x16

	// TypeApplicationData defines a byte value of the TLS record when a
	// peer sends an user data, not a control frames.
	TypeApplicationData byte = 0x17

	// Version10 defines a TLS1.0.
	Version10 uint16 = 769 // 0x03 0x01

	// Version11 defines a TLS1.1.
	Version11 uint16 = 770 // 0x03 0x02

	// Version12 defines a TLS1.2.
	Version12 uint16 = 771 // 0x03 0x03

	// Version13 defines a TLS1.3.
	Version13 uint16 = 772 // 0x03 0x04
)

var (
	emptyRandom       = make([]byte, 32)
	serverHelloSuffix = []byte{
		0x00,       // no compression
		0x00, 0x2e, // 46 bytes of data
		0x00, 0x2b, // Extension - Supported Versions
		0x00, 0x02, // 2 bytes are following
		0x03, 0x04, // TLS 1.3
		0x00, 0x33, // Extension - Key Share
		0x00, 0x24, // 36 bytes
		0x00, 0x1d, // x25519 curve
		0x00, 0x20, // 32 bytes of key
	}
	serverChangeCipherSpec = []byte{
		0x14,       // record type ChangeCipherSpec
		0x03, 0x03, // v1.2
		0x00, 0x01, // payload length (1, big endian)
		0x01, // payload - magic
	}
)

func FakeTLSHandshake(ctx context.Context, conn net.Conn, secrets []*Secret, replay replay.Filter) (secretIndex int, fakeTLSConn *FakeTLSConn, err error) {
	_record := buf.StackNew()
	defer common.KeepAlive(_record)
	record := common.Dup(_record)
	defer record.Release()

	/*var recordHeaderLen int
	recordHeaderLen+=1 // type
	recordHeaderLen+=2 // version
	recordHeaderLen+=2 // payload length*/
	_, err = record.ReadFullFrom(conn, 5)
	if err != nil {
		err = E.Cause(err, "read FakeTLS record")
		return
	}

	recordType := record.Byte(0)
	switch recordType {
	case TypeChangeCipherSpec, TypeHandshake, TypeApplicationData:
	default:
		err = E.New("unknown record type: ", recordType)
		return
	}

	version := binary.BigEndian.Uint16(record.Range(1, 3))
	switch version {
	case Version10, Version11, Version12, Version13:
	default:
		err = E.New("unknown TLS version: ", version)
		return
	}

	length := int(binary.BigEndian.Uint16(record.Range(3, 5)))
	record.Reset()
	_, err = record.ReadFullFrom(conn, length)
	if err != nil {
		err = E.Cause(err, "read FakeTLS record")
		return
	}

	var clientHello *ClientHello
	var foundSecret *Secret
	for i, secret := range secrets {
		clientHello, err = parseClientHello(secret, record)
		if err != nil {
			continue
		}
		err = clientHello.Valid(secret.Host, time.Minute)
		if err != nil {
			continue
		}
		secretIndex = i
		foundSecret = secret
		break
	}
	if foundSecret == nil {
		err = E.New("bad request")
		return
	}

	if !replay.Check(clientHello.SessionID) {
		err = E.New("replay attack detected: ", hex.EncodeToString(clientHello.SessionID))
		return
	}

	_serverHello := buf.StackNew()
	defer common.KeepAlive(_serverHello)
	serverHello := common.Dup(_serverHello)
	defer serverHello.Release()

	generateServerHello(serverHello, clientHello)
	common.Must1(serverHello.Write(serverChangeCipherSpec))

	mac := hmac.New(sha256.New, foundSecret.Key[:])
	mac.Write(clientHello.Random[:])

	appDataHeader := serverHello.Extend(5)
	appDataRandomLen := 1024 + mrand.Intn(3092)
	appDataHeader[0] = TypeApplicationData
	appDataHeader[1] = 0x03 // v1.2
	appDataHeader[2] = 0x03 // v1.2
	binary.BigEndian.PutUint16(appDataHeader[3:], uint16(appDataRandomLen))
	serverHello.WriteRandom(appDataRandomLen)

	mac.Write(serverHello.Bytes())
	copy(serverHello.From(11), mac.Sum(nil))

	_, err = serverHello.WriteTo(conn)
	if err != nil {
		return
	}
	fakeTLSConn = &FakeTLSConn{Conn: conn}
	return
}

type ClientHello struct {
	Time        time.Time
	Random      [32]byte
	SessionID   []byte
	Host        string
	CipherSuite uint16
}

func (c *ClientHello) Valid(hostname string, tolerateTimeSkewness time.Duration) error {
	if c.Host != "" && c.Host != hostname {
		return E.New("incorrect hostname: ", hostname)
	}

	now := time.Now()

	timeDiff := now.Sub(c.Time)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	if timeDiff > tolerateTimeSkewness {
		return E.New("incorrect timestamp. got=",
			c.Time.Unix(), ",now= ", now.Unix(), ", diff=", timeDiff.String())
	}

	return nil
}

func parseClientHello(secret *Secret, handshake *buf.Buffer) (*ClientHello, error) {
	l := handshake.Len()
	if l < 6 { // minimum client hello length
		return nil, E.New("client hello too short: ", l)
	}
	if t := handshake.Byte(0); t != 0x01 { // handshake type client
		return nil, E.New("unknown handshake type: ", t)
	}

	handshakeLen := int(binary.BigEndian.Uint32([]byte{0, handshake.Byte(1), handshake.Byte(2), handshake.Byte(3)}))
	if l-4 != handshakeLen {
		return nil, E.New("incorrect handshake size. manifested=", handshakeLen, ", got=", l-4)
	}

	hello := &ClientHello{}

	mac := hmac.New(sha256.New, secret.Key[:])
	mac.Write([]byte{TypeHandshake, 0x03, 0x01})
	var payloadLen [2]byte
	binary.BigEndian.PutUint16(payloadLen[:], uint16(l))
	mac.Write(payloadLen[:])
	mac.Write(handshake.Range(0, 6))
	mac.Write(emptyRandom)
	mac.Write(handshake.From(38))
	computedRandom := mac.Sum(nil)
	for i := 0; i < 32; i++ {
		computedRandom[i] ^= handshake.Byte(6 + i)
	}
	if subtle.ConstantTimeCompare(emptyRandom[:32-4], computedRandom[:32-4]) != 1 {
		return nil, E.New("bad digest")
	}

	timestamp := int64(binary.LittleEndian.Uint32(computedRandom[32-4:]))
	hello.Time = time.Unix(timestamp, 0)

	copy(hello.Random[:], handshake.Range(6, 38))

	parseSessionID(hello, handshake)
	parseCipherSuite(hello, handshake)
	parseSNI(hello, handshake.Bytes())

	return hello, nil
}

func parseSessionID(hello *ClientHello, handshake *buf.Buffer) {
	hello.SessionID = make([]byte, handshake.Byte(38))
	copy(hello.SessionID, handshake.From(38+1))
}

func parseCipherSuite(hello *ClientHello, handshake *buf.Buffer) {
	cipherSuiteOffset := 38 + len(hello.SessionID) + 3 //nolint: gomnd
	hello.CipherSuite = binary.BigEndian.Uint16(handshake.Range(cipherSuiteOffset, cipherSuiteOffset+2))
}

func parseSNI(hello *ClientHello, handshake []byte) {
	cipherSuiteOffset := 38 + len(hello.SessionID) + 1
	handshake = handshake[cipherSuiteOffset:]

	cipherSuiteLength := binary.BigEndian.Uint16(handshake[:2])
	handshake = handshake[2+cipherSuiteLength:]

	compressionMethodsLength := int(handshake[0])
	handshake = handshake[1+compressionMethodsLength:]

	extensionsLength := binary.BigEndian.Uint16(handshake[:2])
	handshake = handshake[2 : 2+extensionsLength]

	for len(handshake) > 0 {
		if binary.BigEndian.Uint16(handshake[:2]) != 0x00 { // extension SNI
			extensionsLength := binary.BigEndian.Uint16(handshake[2:4])
			handshake = handshake[4+extensionsLength:]

			continue
		}

		hostnameLength := binary.BigEndian.Uint16(handshake[7:9])
		handshake = handshake[9:]
		hello.Host = string(handshake[:int(hostnameLength)])

		return
	}
}

func generateServerHello(record *buf.Buffer, ch *ClientHello) {
	common.Must1(record.Write([]byte{0x03, 0x03})) // v1.2
	common.Must(record.WriteZeroN(32))
	common.Must(record.WriteByte(byte(len(ch.SessionID))))
	common.Must1(record.Write(ch.SessionID))
	binary.BigEndian.PutUint16(record.Extend(2), ch.CipherSuite)
	common.Must1(record.Write(serverHelloSuffix))

	scalar := buf.Get(32)
	defer buf.Put(scalar)
	common.Must1(rand.Read(scalar))
	curve, _ := curve25519.X25519(scalar, curve25519.Basepoint)
	common.Must1(record.Write(curve))

	l := record.Len()
	header := record.ExtendHeader(4)
	binary.BigEndian.PutUint32(header, uint32(l))
	header[0] = 0x02 // handshake type server

	l = record.Len()
	header = record.ExtendHeader(5)
	header[0] = TypeHandshake
	header[1] = 0x03 // v1.2
	header[2] = 0x03 // v1.2
	binary.BigEndian.PutUint16(header[3:], uint16(l))
}
