package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"filippo.io/edwards25519"
)

type KeyPair struct {
	Private *edwards25519.Scalar
	Public  *edwards25519.Point
}

func GenerateMasterKey() (*KeyPair, error) {
	var seed [64]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return nil, err
	}

	private, err := edwards25519.NewScalar().SetUniformBytes(seed[:])
	if err != nil {
		return nil, err
	}
	public := new(edwards25519.Point).ScalarBaseMult(private)
	return &KeyPair{Private: private, Public: public}, nil
}

func SplitPrivateKey(master *edwards25519.Scalar) (string, error) {
	var seed [64]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return "", err
	}
	r, err := edwards25519.NewScalar().SetUniformBytes(seed[:])
	if err != nil {
		return "", err
	}
	k := new(edwards25519.Scalar).Subtract(master, r)

	full := make([]byte, 64)
	copy(full[:32], r.Bytes())
	copy(full[32:], k.Bytes())
	return hex.EncodeToString(full), nil
}

// RecoverPublicKey takes either a master private scalar (32 bytes hex) or a split private key (64 bytes hex, r||k)
// and returns the corresponding public key point.
func RecoverPublicKey(keyHex string) (*edwards25519.Point, error) {
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	switch len(keyBytes) {
	case 32:
		private, err := edwards25519.NewScalar().SetCanonicalBytes(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid scalar: %w", err)
		}
		return new(edwards25519.Point).ScalarBaseMult(private), nil
	case 64:
		rBytes := keyBytes[:32]
		kBytes := keyBytes[32:]

		r, err := edwards25519.NewScalar().SetCanonicalBytes(rBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid scalar r: %w", err)
		}
		k, err := edwards25519.NewScalar().SetCanonicalBytes(kBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid scalar k: %w", err)
		}
		sum := new(edwards25519.Scalar).Add(r, k)
		return new(edwards25519.Point).ScalarBaseMult(sum), nil
	default:
		return nil, errors.New("invalid key length: must be 32 bytes (Master) or 64 bytes (Split)")
	}
}

func EncodePoint(p *edwards25519.Point) string {
	return hex.EncodeToString(p.Bytes())
}

