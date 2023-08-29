//go:build with_ech

package tls

import (
	"bytes"
	"encoding/binary"
	"encoding/pem"

	cftls "github.com/sagernet/cloudflare-tls"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/cloudflare/circl/hpke"
	"github.com/cloudflare/circl/kem"
)

func ECHKeygenDefault(serverName string, pqSignatureSchemesEnabled bool) (configPem string, keyPem string, err error) {
	cipherSuites := []echCipherSuite{
		{
			kdf:  hpke.KDF_HKDF_SHA256,
			aead: hpke.AEAD_AES128GCM,
		}, {
			kdf:  hpke.KDF_HKDF_SHA256,
			aead: hpke.AEAD_ChaCha20Poly1305,
		},
	}

	keyConfig := []myECHKeyConfig{
		{id: 0, kem: hpke.KEM_X25519_HKDF_SHA256},
	}
	if pqSignatureSchemesEnabled {
		keyConfig = append(keyConfig, myECHKeyConfig{id: 1, kem: hpke.KEM_X25519_KYBER768_DRAFT00})
	}

	keyPairs, err := echKeygen(0xfe0d, serverName, keyConfig, cipherSuites)
	if err != nil {
		return
	}

	var configBuffer bytes.Buffer
	var totalLen uint16
	for _, keyPair := range keyPairs {
		totalLen += uint16(len(keyPair.rawConf))
	}
	binary.Write(&configBuffer, binary.BigEndian, totalLen)
	for _, keyPair := range keyPairs {
		configBuffer.Write(keyPair.rawConf)
	}

	var keyBuffer bytes.Buffer
	for _, keyPair := range keyPairs {
		keyBuffer.Write(keyPair.rawKey)
	}

	configPem = string(pem.EncodeToMemory(&pem.Block{Type: "ECH CONFIGS", Bytes: configBuffer.Bytes()}))
	keyPem = string(pem.EncodeToMemory(&pem.Block{Type: "ECH KEYS", Bytes: keyBuffer.Bytes()}))
	return
}

type echKeyConfigPair struct {
	id      uint8
	key     cftls.EXP_ECHKey
	rawKey  []byte
	conf    myECHKeyConfig
	rawConf []byte
}

type echCipherSuite struct {
	kdf  hpke.KDF
	aead hpke.AEAD
}

type myECHKeyConfig struct {
	id   uint8
	kem  hpke.KEM
	seed []byte
}

func echKeygen(version uint16, serverName string, conf []myECHKeyConfig, suite []echCipherSuite) ([]echKeyConfigPair, error) {
	be := binary.BigEndian
	// prepare for future update
	if version != 0xfe0d {
		return nil, E.New("unsupported ECH version", version)
	}

	suiteBuf := make([]byte, 0, len(suite)*4+2)
	suiteBuf = be.AppendUint16(suiteBuf, uint16(len(suite))*4)
	for _, s := range suite {
		if !s.kdf.IsValid() || !s.aead.IsValid() {
			return nil, E.New("invalid HPKE cipher suite")
		}
		suiteBuf = be.AppendUint16(suiteBuf, uint16(s.kdf))
		suiteBuf = be.AppendUint16(suiteBuf, uint16(s.aead))
	}

	pairs := []echKeyConfigPair{}
	for _, c := range conf {
		pair := echKeyConfigPair{}
		pair.id = c.id
		pair.conf = c

		if !c.kem.IsValid() {
			return nil, E.New("invalid HPKE KEM")
		}

		kpGenerator := c.kem.Scheme().GenerateKeyPair
		if len(c.seed) > 0 {
			kpGenerator = func() (kem.PublicKey, kem.PrivateKey, error) {
				pub, sec := c.kem.Scheme().DeriveKeyPair(c.seed)
				return pub, sec, nil
			}
			if len(c.seed) < c.kem.Scheme().PrivateKeySize() {
				return nil, E.New("HPKE KEM seed too short")
			}
		}

		pub, sec, err := kpGenerator()
		if err != nil {
			return nil, E.Cause(err, "generate ECH config key pair")
		}
		b := []byte{}
		b = be.AppendUint16(b, version)
		b = be.AppendUint16(b, 0) // length field
		// contents
		// key config
		b = append(b, c.id)
		b = be.AppendUint16(b, uint16(c.kem))
		pubBuf, err := pub.MarshalBinary()
		if err != nil {
			return nil, E.Cause(err, "serialize ECH public key")
		}
		b = be.AppendUint16(b, uint16(len(pubBuf)))
		b = append(b, pubBuf...)

		b = append(b, suiteBuf...)
		// end key config
		// max name len, not supported
		b = append(b, 0)
		// server name
		b = append(b, byte(len(serverName)))
		b = append(b, []byte(serverName)...)
		// extensions, not supported
		b = be.AppendUint16(b, 0)

		be.PutUint16(b[2:], uint16(len(b)-4))

		pair.rawConf = b

		secBuf, err := sec.MarshalBinary()
		sk := []byte{}
		sk = be.AppendUint16(sk, uint16(len(secBuf)))
		sk = append(sk, secBuf...)
		sk = be.AppendUint16(sk, uint16(len(b)))
		sk = append(sk, b...)

		cfECHKeys, err := cftls.EXP_UnmarshalECHKeys(sk)
		if err != nil {
			return nil, E.Cause(err, "bug: can't parse generated ECH server key")
		}
		if len(cfECHKeys) != 1 {
			return nil, E.New("bug: unexpected server key count")
		}
		pair.key = cfECHKeys[0]
		pair.rawKey = sk

		pairs = append(pairs, pair)
	}
	return pairs, nil
}
