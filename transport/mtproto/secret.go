package mtproto

import (
	"encoding/base64"
	"encoding/hex"

	E "github.com/sagernet/sing/common/exceptions"
)

// mod from https://github.com/9seconds/mtg/blob/master/mtglib/secret.go

const (
	secretFakeTLSFirstByte byte = 0xEE
	secretKeyLength        int  = 16
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
	Key  [secretKeyLength]byte
	Host string
}

func (s *Secret) String() string {
	data := append([]byte{secretFakeTLSFirstByte}, s.Key[:]...)
	data = append(data, s.Host...)
	return hex.EncodeToString(data)
}

func ParseSecret(plainText string) (*Secret, error) {
	decoded, err := hex.DecodeString(plainText)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(plainText)
	}
	if err != nil {
		return nil, E.Cause(err, "bad secret format")
	}
	if len(decoded) < 2 {
		return nil, E.New("secret is truncated, length=", len(decoded))
	}
	if decoded[0] != secretFakeTLSFirstByte || len(decoded) < 1+secretKeyLength {
		return nil, E.New("bad FakeTLS secret")
	}
	var secret Secret
	copy(secret.Key[:], decoded[1:secretKeyLength+1])
	secret.Host = string(decoded[1+secretKeyLength:])
	if secret.Host == "" {
		return nil, E.New("bad FakeTLS secret: empty server host")
	}
	return &secret, nil
}
