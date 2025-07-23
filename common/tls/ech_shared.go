package tls

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/pem"

	"golang.org/x/crypto/cryptobyte"
)

type ECHCapableConfig interface {
	Config
	ECHConfigList() []byte
	SetECHConfigList([]byte)
}

func ECHKeygenDefault(publicName string) (configPem string, keyPem string, err error) {
	echKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return
	}
	echConfig, err := marshalECHConfig(0, echKey.PublicKey().Bytes(), publicName, 0)
	if err != nil {
		return
	}
	configBuilder := cryptobyte.NewBuilder(nil)
	configBuilder.AddUint16LengthPrefixed(func(builder *cryptobyte.Builder) {
		builder.AddBytes(echConfig)
	})
	configBytes, err := configBuilder.Bytes()
	if err != nil {
		return
	}
	keyBuilder := cryptobyte.NewBuilder(nil)
	keyBuilder.AddUint16LengthPrefixed(func(builder *cryptobyte.Builder) {
		builder.AddBytes(echKey.Bytes())
	})
	keyBuilder.AddUint16LengthPrefixed(func(builder *cryptobyte.Builder) {
		builder.AddBytes(echConfig)
	})
	keyBytes, err := keyBuilder.Bytes()
	if err != nil {
		return
	}
	configPem = string(pem.EncodeToMemory(&pem.Block{Type: "ECH CONFIGS", Bytes: configBytes}))
	keyPem = string(pem.EncodeToMemory(&pem.Block{Type: "ECH KEYS", Bytes: keyBytes}))
	return
}

func marshalECHConfig(id uint8, pubKey []byte, publicName string, maxNameLen uint8) ([]byte, error) {
	const extensionEncryptedClientHello = 0xfe0d
	const DHKEM_X25519_HKDF_SHA256 = 0x0020
	const KDF_HKDF_SHA256 = 0x0001
	builder := cryptobyte.NewBuilder(nil)
	builder.AddUint16(extensionEncryptedClientHello)
	builder.AddUint16LengthPrefixed(func(builder *cryptobyte.Builder) {
		builder.AddUint8(id)

		builder.AddUint16(DHKEM_X25519_HKDF_SHA256) // The only DHKEM we support
		builder.AddUint16LengthPrefixed(func(builder *cryptobyte.Builder) {
			builder.AddBytes(pubKey)
		})
		builder.AddUint16LengthPrefixed(func(builder *cryptobyte.Builder) {
			const (
				AEAD_AES_128_GCM      = 0x0001
				AEAD_AES_256_GCM      = 0x0002
				AEAD_ChaCha20Poly1305 = 0x0003
			)
			for _, aeadID := range []uint16{AEAD_AES_128_GCM, AEAD_AES_256_GCM, AEAD_ChaCha20Poly1305} {
				builder.AddUint16(KDF_HKDF_SHA256) // The only KDF we support
				builder.AddUint16(aeadID)
			}
		})
		builder.AddUint8(maxNameLen)
		builder.AddUint8LengthPrefixed(func(builder *cryptobyte.Builder) {
			builder.AddBytes([]byte(publicName))
		})
		builder.AddUint16(0) // extensions
	})
	return builder.Bytes()
}
