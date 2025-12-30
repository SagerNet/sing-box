package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"golang.org/x/crypto/chacha20poly1305"
)

type AEADConn struct {
	net.Conn
	aead      cipher.AEAD
	readBuf   bytes.Buffer
	nonceSize int
}

func NewAEADConn(c net.Conn, key string, method string) (*AEADConn, error) {
	if method == "none" {
		return &AEADConn{Conn: c, aead: nil}, nil
	}

	hash := sha256.Sum256([]byte(key))
	keyBytes := hash[:]

	var (
		aead cipher.AEAD
		err  error
	)
	switch method {
	case "aes-128-gcm":
		block, _ := aes.NewCipher(keyBytes[:16])
		aead, err = cipher.NewGCM(block)
	case "chacha20-poly1305":
		aead, err = chacha20poly1305.New(keyBytes)
	default:
		return nil, fmt.Errorf("unsupported cipher: %s", method)
	}
	if err != nil {
		return nil, err
	}

	return &AEADConn{
		Conn:      c,
		aead:      aead,
		nonceSize: aead.NonceSize(),
	}, nil
}

func (c *AEADConn) Write(p []byte) (int, error) {
	if c.aead == nil {
		return c.Conn.Write(p)
	}

	// 2-byte length prefix (uint16), then nonce+ciphertext.
	maxPayload := 65535 - c.nonceSize - c.aead.Overhead()
	totalWritten := 0
	var frameBuf bytes.Buffer
	header := make([]byte, 2)
	nonce := make([]byte, c.nonceSize)

	for len(p) > 0 {
		chunkSize := len(p)
		if chunkSize > maxPayload {
			chunkSize = maxPayload
		}
		chunk := p[:chunkSize]
		p = p[chunkSize:]

		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return totalWritten, err
		}

		ciphertext := c.aead.Seal(nil, nonce, chunk, nil)
		frameLen := len(nonce) + len(ciphertext)
		binary.BigEndian.PutUint16(header, uint16(frameLen))

		frameBuf.Reset()
		frameBuf.Write(header)
		frameBuf.Write(nonce)
		frameBuf.Write(ciphertext)

		if _, err := c.Conn.Write(frameBuf.Bytes()); err != nil {
			return totalWritten, err
		}
		totalWritten += chunkSize
	}
	return totalWritten, nil
}

func (c *AEADConn) Read(p []byte) (int, error) {
	if c.aead == nil {
		return c.Conn.Read(p)
	}

	if c.readBuf.Len() > 0 {
		return c.readBuf.Read(p)
	}

	header := make([]byte, 2)
	if _, err := io.ReadFull(c.Conn, header); err != nil {
		return 0, err
	}
	frameLen := int(binary.BigEndian.Uint16(header))

	body := make([]byte, frameLen)
	if _, err := io.ReadFull(c.Conn, body); err != nil {
		return 0, err
	}

	if len(body) < c.nonceSize {
		return 0, errors.New("frame too short")
	}
	nonce := body[:c.nonceSize]
	ciphertext := body[c.nonceSize:]

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return 0, errors.New("decryption failed")
	}

	c.readBuf.Write(plaintext)
	return c.readBuf.Read(p)
}

