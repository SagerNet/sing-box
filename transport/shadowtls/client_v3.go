package shadowtls

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"io"
	"net"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

const (
	tlsRandomSize    = 32
	tlsHeaderSize    = 5
	tlsSessionIDSize = 32

	clientHello = 1
	serverHello = 2

	changeCipherSpec = 20
	alert            = 21
	handshake        = 22
	applicationData  = 23

	serverRandomIndex    = tlsHeaderSize + 1 + 3 + 2
	sessionIDLengthIndex = tlsHeaderSize + 1 + 3 + 2 + tlsRandomSize
	tlsHmacHeaderSize    = tlsHeaderSize + hmacSize
	hmacSize             = 4
)

func generateSessionID(password string) func(clientHello []byte, sessionID []byte) error {
	return func(clientHello []byte, sessionID []byte) error {
		const sessionIDStart = 1 + 3 + 2 + tlsRandomSize + 1
		if len(clientHello) < sessionIDStart+tlsSessionIDSize {
			return E.New("unexpected client hello length")
		}
		_, err := rand.Read(sessionID[:tlsSessionIDSize-hmacSize])
		if err != nil {
			return err
		}
		hmacSHA1Hash := hmac.New(sha1.New, []byte(password))
		hmacSHA1Hash.Write(clientHello[:sessionIDStart])
		hmacSHA1Hash.Write(sessionID)
		hmacSHA1Hash.Write(clientHello[sessionIDStart+tlsSessionIDSize:])
		copy(sessionID[tlsSessionIDSize-hmacSize:], hmacSHA1Hash.Sum(nil)[:hmacSize])
		return nil
	}
}

type StreamWrapper struct {
	net.Conn
	password     string
	buffer       *buf.Buffer
	serverRandom []byte
	readHMAC     hash.Hash
	readHMACKey  []byte
	authorized   bool
}

func NewStreamWrapper(conn net.Conn, password string) *StreamWrapper {
	return &StreamWrapper{
		Conn:     conn,
		password: password,
	}
}

func (w *StreamWrapper) Authorized() (bool, []byte, hash.Hash) {
	return w.authorized, w.serverRandom, w.readHMAC
}

func (w *StreamWrapper) Read(p []byte) (n int, err error) {
	if w.buffer != nil {
		if !w.buffer.IsEmpty() {
			return w.buffer.Read(p)
		}
		w.buffer.Release()
		w.buffer = nil
	}
	var tlsHeader [tlsHeaderSize]byte
	_, err = io.ReadFull(w.Conn, tlsHeader[:])
	if err != nil {
		return
	}
	length := int(binary.BigEndian.Uint16(tlsHeader[3:tlsHeaderSize]))
	w.buffer = buf.NewSize(tlsHeaderSize + length)
	common.Must1(w.buffer.Write(tlsHeader[:]))
	_, err = w.buffer.ReadFullFrom(w.Conn, length)
	if err != nil {
		return
	}
	buffer := w.buffer.Bytes()
	switch tlsHeader[0] {
	case handshake:
		if len(buffer) > serverRandomIndex+tlsRandomSize && buffer[5] == serverHello {
			w.serverRandom = make([]byte, tlsRandomSize)
			copy(w.serverRandom, buffer[serverRandomIndex:serverRandomIndex+tlsRandomSize])
			w.readHMAC = hmac.New(sha1.New, []byte(w.password))
			w.readHMAC.Write(w.serverRandom)
			w.readHMACKey = kdf(w.password, w.serverRandom)
		}
	case applicationData:
		w.authorized = false
		if len(buffer) > tlsHmacHeaderSize && w.readHMAC != nil {
			w.readHMAC.Write(buffer[tlsHmacHeaderSize:])
			if hmac.Equal(w.readHMAC.Sum(nil)[:hmacSize], buffer[tlsHeaderSize:tlsHmacHeaderSize]) {
				xorSlice(buffer[tlsHmacHeaderSize:], w.readHMACKey)
				copy(buffer[hmacSize:], buffer[:tlsHeaderSize])
				binary.BigEndian.PutUint16(buffer[hmacSize+3:], uint16(len(buffer)-tlsHmacHeaderSize))
				w.buffer.Advance(hmacSize)
				w.authorized = true
			}
		}
	}
	return w.buffer.Read(p)
}

func kdf(password string, serverRandom []byte) []byte {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	hasher.Write(serverRandom)
	return hasher.Sum(nil)
}

func xorSlice(data []byte, key []byte) {
	for i := range data {
		data[i] ^= key[i%len(key)]
	}
}

var _ N.VectorisedWriter = (*VerifiedConn)(nil)

type VerifiedConn struct {
	net.Conn
	writer     N.VectorisedWriter
	hmacAdd    hash.Hash
	hmacVerify hash.Hash
	hmacIgnore hash.Hash

	buffer *buf.Buffer
}

func NewVerifiedConn(
	conn net.Conn,
	hmacAdd hash.Hash,
	hmacVerify hash.Hash,
	hmacIgnore hash.Hash,
) *VerifiedConn {
	return &VerifiedConn{
		Conn:       conn,
		writer:     bufio.NewVectorisedWriter(conn),
		hmacAdd:    hmacAdd,
		hmacVerify: hmacVerify,
		hmacIgnore: hmacIgnore,
	}
}

func (c *VerifiedConn) Read(b []byte) (n int, err error) {
	if c.buffer != nil {
		if !c.buffer.IsEmpty() {
			return c.buffer.Read(b)
		}
		c.buffer.Release()
		c.buffer = nil
	}
	for {
		var tlsHeader [tlsHeaderSize]byte
		_, err = io.ReadFull(c.Conn, tlsHeader[:])
		if err != nil {
			sendAlert(c.Conn)
			return
		}
		length := int(binary.BigEndian.Uint16(tlsHeader[3:tlsHeaderSize]))
		c.buffer = buf.NewSize(tlsHeaderSize + length)
		common.Must1(c.buffer.Write(tlsHeader[:]))
		_, err = c.buffer.ReadFullFrom(c.Conn, length)
		if err != nil {
			return
		}
		buffer := c.buffer.Bytes()
		switch buffer[0] {
		case alert:
			err = E.Cause(net.ErrClosed, "remote alert")
			return
		case applicationData:
			if c.hmacIgnore != nil {
				if verifyApplicationData(buffer, c.hmacIgnore, false) {
					c.buffer.Release()
					c.buffer = nil
					continue
				} else {
					c.hmacIgnore = nil
				}
			}
			if !verifyApplicationData(buffer, c.hmacVerify, true) {
				sendAlert(c.Conn)
				err = E.New("application data verification failed")
				return
			}
			c.buffer.Advance(tlsHmacHeaderSize)
		default:
			sendAlert(c.Conn)
			err = E.New("unexpected TLS record type: ", buffer[0])
			return
		}
		return c.buffer.Read(b)
	}
}

func (c *VerifiedConn) Write(p []byte) (n int, err error) {
	pTotal := len(p)
	for len(p) > 0 {
		var pWrite []byte
		if len(p) > 16384 {
			pWrite = p[:16384]
			p = p[16384:]
		} else {
			pWrite = p
			p = nil
		}
		_, err = c.write(pWrite)
	}
	if err == nil {
		n = pTotal
	}
	return
}

func (c *VerifiedConn) write(p []byte) (n int, err error) {
	var header [tlsHmacHeaderSize]byte
	header[0] = applicationData
	header[1] = 3
	header[2] = 3
	binary.BigEndian.PutUint16(header[3:tlsHeaderSize], hmacSize+uint16(len(p)))
	c.hmacAdd.Write(p)
	hmacHash := c.hmacAdd.Sum(nil)[:hmacSize]
	c.hmacAdd.Write(hmacHash)
	copy(header[tlsHeaderSize:], hmacHash)
	_, err = bufio.WriteVectorised(c.writer, [][]byte{common.Dup(header[:]), p})
	if err == nil {
		n = len(p)
	}
	return
}

func (c *VerifiedConn) WriteVectorised(buffers []*buf.Buffer) error {
	var header [tlsHmacHeaderSize]byte
	header[0] = applicationData
	header[1] = 3
	header[2] = 3
	binary.BigEndian.PutUint16(header[3:tlsHeaderSize], hmacSize+uint16(buf.LenMulti(buffers)))
	for _, buffer := range buffers {
		c.hmacAdd.Write(buffer.Bytes())
	}
	c.hmacAdd.Write(c.hmacAdd.Sum(nil)[:hmacSize])
	copy(header[tlsHeaderSize:], c.hmacAdd.Sum(nil)[:hmacSize])
	return c.writer.WriteVectorised(append([]*buf.Buffer{buf.As(header[:])}, buffers...))
}

func verifyApplicationData(frame []byte, hmac hash.Hash, update bool) bool {
	if frame[1] != 3 || frame[2] != 3 || len(frame) < tlsHmacHeaderSize {
		return false
	}
	hmac.Write(frame[tlsHmacHeaderSize:])
	hmacHash := hmac.Sum(nil)[:hmacSize]
	if update {
		hmac.Write(hmacHash)
	}
	return bytes.Equal(frame[tlsHeaderSize:tlsHeaderSize+hmacSize], hmacHash)
}

func sendAlert(writer io.Writer) {
	const recordSize = 31
	record := [recordSize]byte{
		alert,
		3,
		3,
		0,
		recordSize - tlsHeaderSize,
	}
	_, err := rand.Read(record[tlsHeaderSize:])
	if err != nil {
		return
	}
	writer.Write(record[:])
}
