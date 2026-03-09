package v2raykcp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"hash/fnv"
)

// SimpleAuthenticator is a legacy AEAD used for KCP encryption.
type SimpleAuthenticator struct{}

// NewSimpleAuthenticator creates a new SimpleAuthenticator
func NewSimpleAuthenticator() cipher.AEAD {
	return &SimpleAuthenticator{}
}

// NonceSize implements cipher.AEAD.NonceSize().
func (*SimpleAuthenticator) NonceSize() int {
	return 0
}

// Overhead implements cipher.AEAD.Overhead().
func (*SimpleAuthenticator) Overhead() int {
	return 6
}

// Seal implements cipher.AEAD.Seal().
func (a *SimpleAuthenticator) Seal(dst, nonce, plain, extra []byte) []byte {
	dst = append(dst, 0, 0, 0, 0, 0, 0) // 4 bytes for hash, and then 2 bytes for length
	binary.BigEndian.PutUint16(dst[4:], uint16(len(plain)))
	dst = append(dst, plain...)

	fnvHash := fnv.New32a()
	fnvHash.Write(dst[4:])
	fnvHash.Sum(dst[:0])

	dstLen := len(dst)
	xtra := 4 - dstLen%4
	if xtra != 4 {
		dst = append(dst, make([]byte, xtra)...)
	}
	xorfwd(dst)
	if xtra != 4 {
		dst = dst[:dstLen]
	}
	return dst
}

// Open implements cipher.AEAD.Open().
func (a *SimpleAuthenticator) Open(dst, nonce, cipherText, extra []byte) ([]byte, error) {
	dst = append(dst, cipherText...)
	dstLen := len(dst)
	xtra := 4 - dstLen%4
	if xtra != 4 {
		dst = append(dst, make([]byte, xtra)...)
	}
	xorbkd(dst)
	if xtra != 4 {
		dst = dst[:dstLen]
	}

	fnvHash := fnv.New32a()
	fnvHash.Write(dst[4:])
	if binary.BigEndian.Uint32(dst[:4]) != fnvHash.Sum32() {
		return nil, newError("invalid auth")
	}

	length := binary.BigEndian.Uint16(dst[4:6])
	if len(dst)-6 != int(length) {
		return nil, newError("invalid auth")
	}

	return dst[6:], nil
}

// xorfwd performs XOR forwards in words, x[i] ^= x[i-4], i from 0 to len.
func xorfwd(b []byte) {
	for i := 4; i < len(b); i++ {
		b[i] ^= b[i-4]
	}
}

// xorbkd performs XOR backwards in words, x[i] ^= x[i-4], i from len to 0.
func xorbkd(b []byte) {
	for i := len(b) - 1; i >= 4; i-- {
		b[i] ^= b[i-4]
	}
}

// NewAEADAESGCMBasedOnSeed creates a new AES-GCM AEAD based on a seed
func NewAEADAESGCMBasedOnSeed(seed string) cipher.AEAD {
	// Use SHA256 to hash the seed
	hashedSeed := sha256.Sum256([]byte(seed))
	
	// Use first 16 bytes as AES-128 key
	block, err := aes.NewCipher(hashedSeed[:16])
	if err != nil {
		panic(err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	
	return gcm
}
