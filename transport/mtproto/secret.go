package mtproto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

// kanged from https://github.com/9seconds/mtg/blob/master/mtglib/secret.go

const (
	secretFakeTLSFirstByte byte = 0xEE

	SecretKeyLength = 16
)

var (
	secretEmptyKey [SecretKeyLength]byte

	ErrSecretEmpty = E.New("mtproto: secret is empty")
)

// Secret is a data structure that presents a secret.
//
// Telegram secret is not a simple string like
// "ee367a189aee18fa31c190054efd4a8e9573746f726167652e676f6f676c65617069732e636f6d".
// Actually, this is a serialized datastructure of 2 parts: key and host.
//
//	ee367a189aee18fa31c190054efd4a8e9573746f726167652e676f6f676c65617069732e636f6d
//	|-|-------------------------------|-------------------------------------------
//	p key                             hostname
//
// Serialized secret starts with 'ee'. Actually, in the past we also had 'dd'
// secrets and prefixless ones. But this is history. Currently, we do have only
// 'ee' secrets which mean faketls + protection from statistical attacks on a
// length. 'ee' is a byte 238 (0xee).
//
// After that, we have 16 bytes of the key. This is a random generated secret
// data of the proxy and this data is used to derive authentication schemas.
// These secrets are mixed into hmacs and sha256 checksums which are used to
// build AEAD ciphers for obfuscated2 protocol and ensure faketls handshake.
//
// Host is a domain fronting hostname in latin1 (ASCII) encoding. This hostname
// should be used for SNI in faketls and sing-box verifies it. Also, this is when
// sing-box gets about a domain fronting hostname.
//
// Secrets can be serialized into 2 forms: hex and base64. If you decode both
// forms into bytes, you'll get the same byte array. Telegram clients nowadays
// accept all forms.
type Secret struct {
	// Key is a set of bytes used for traffic authentication.
	Key [SecretKeyLength]byte

	// Host is a domain fronting hostname.
	Host string
}

func (s *Secret) Set(text string) error {
	if text == "" {
		return ErrSecretEmpty
	}

	decoded, err := hex.DecodeString(text)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(text)
	}

	if err != nil {
		return E.New("incorrect secret format: ", err)
	}

	l := len(decoded)
	if l < 2 { //nolint: gomnd // we need at least 1 byte here
		return E.New("secret is truncated, length=", l)
	}

	if decoded[0] != secretFakeTLSFirstByte {
		return E.New("incorrect first byte of secret: ", decoded[0])
	}

	if l < 1+SecretKeyLength { // 1 for FakeTLS first byte
		return E.New("secret has incorrect length ", len(decoded))
	}

	copy(s.Key[:], decoded[1:SecretKeyLength+1])
	s.Host = string(decoded[1+SecretKeyLength:])

	if s.Host == "" {
		return E.New("hostname cannot be empty: ", text)
	}

	return nil
}

// Valid checks if this secret is valid and can be used in proxy.
func (s Secret) Valid() bool {
	return s.Key != secretEmptyKey && s.Host != ""
}

// String is to support fmt.Stringer interface.
func (s Secret) String() string {
	return s.Base64()
}

// Base64 returns a base64-encoded form of this secret.
func (s Secret) Base64() string {
	return base64.RawURLEncoding.EncodeToString(s.makeBytes())
}

// Hex returns a hex-encoded form of this secret (ee-secret).
func (s Secret) Hex() string {
	return hex.EncodeToString(s.makeBytes())
}

func (s *Secret) makeBytes() []byte {
	data := append([]byte{secretFakeTLSFirstByte}, s.Key[:]...)
	data = append(data, s.Host...)

	return data
}

// GenerateSecret makes a new secret with a given hostname.
func GenerateSecret(hostname string) Secret {
	s := Secret{Host: hostname}
	common.Must1(rand.Read(s.Key[:]))

	return s
}

// ParseSecret parses a secret (both hex and base64 forms).
func ParseSecret(secret string) (Secret, error) {
	s := Secret{}

	return s, s.Set(secret)
}
