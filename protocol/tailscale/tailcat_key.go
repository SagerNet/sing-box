package tailscale

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/crypto/curve25519"
)

// tailcat derives the path-discovery key from the node key instead of
// generating one; both peers must agree on this derivation for the
// server's disco public key in a client config to match what the
// server signs disco frames with.
const tailcatDiscoKeyLabel = "github.com/tailscale/tailcat disco key v1"

func NewTailcatPrivateKey() [32]byte {
	var privateKey [32]byte
	_, err := rand.Read(privateKey[:])
	if err != nil {
		panic(err)
	}
	clampTailcatPrivateKey(&privateKey)
	return privateKey
}

func TailcatPublicKey(privateKey [32]byte) [32]byte {
	publicKey, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		panic(err)
	}
	return [32]byte(publicKey)
}

func TailcatDiscoPrivateKey(privateKey [32]byte) [32]byte {
	mac := hmac.New(sha256.New, privateKey[:])
	mac.Write([]byte(tailcatDiscoKeyLabel))
	discoKey := [32]byte(mac.Sum(nil))
	clampTailcatPrivateKey(&discoKey)
	return discoKey
}

func DecodeTailcatKey(value string) ([32]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return [32]byte{}, E.Cause(err, "decode base64")
	}
	if len(decoded) != 32 {
		return [32]byte{}, E.New("unexpected key length: ", len(decoded))
	}
	return [32]byte(decoded), nil
}

func EncodeTailcatKey(key [32]byte) string {
	return base64.StdEncoding.EncodeToString(key[:])
}

// tailcat addresses live in Tailscale's ULA range with the top 80 bits
// of the node public key as the interface identifier.
func TailcatAddress(publicKey [32]byte) netip.Addr {
	var address [16]byte
	copy(address[:], []byte{0xfd, 0x7a, 0x11, 0x5c, 0xa1, 0xe0})
	copy(address[6:], publicKey[:10])
	return netip.AddrFrom16(address)
}

func clampTailcatPrivateKey(key *[32]byte) {
	key[0] &= 248
	key[31] = (key[31] & 127) | 64
}
