//go:build with_utls

package tls

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"reflect"
	"testing"
	"unsafe"

	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	utls "github.com/metacubex/utls"
	"github.com/stretchr/testify/require"
)

func TestMLDSA65Verifier(t *testing.T) {
	pub, priv, err := mldsa65.GenerateKey(nil)
	require.NoError(t, err)
	pubBinary, _ := pub.MarshalBinary()

	// Mock AuthKey and peer cert public key
	authKey := make([]byte, 32)
	peerPubKey, _, _ := ed25519.GenerateKey(nil)

	helloRaw := make([]byte, 100)
	serverHelloRaw := make([]byte, 100)

	h := hmac.New(sha512.New, authKey)
	h.Write(peerPubKey)
	certSignature := h.Sum(nil)

	h.Write(helloRaw)
	h.Write(serverHelloRaw)
	mldsaMsg := h.Sum(nil)

	scheme := mldsa65.Scheme()
	mldsaSig := scheme.Sign(priv, mldsaMsg, nil)

	cert := &x509.Certificate{
		PublicKey: peerPubKey,
		Signature: certSignature,
		Extensions: []pkix.Extension{
			{
				Id:    []int{1, 2, 3}, // Dummy OID
				Value: mldsaSig,
			},
		},
	}

	verifier := &realityVerifier{
		authKey:       authKey,
		mldsa65Verify: pubBinary,
		UConn: &utls.UConn{
			Conn: &utls.Conn{},
		},
	}

	// Set peerCertificates using unsafe as sing-box does
	p, _ := reflect.TypeOf(verifier.Conn).Elem().FieldByName("peerCertificates")
	*(*([]*x509.Certificate))(unsafe.Pointer(uintptr(unsafe.Pointer(verifier.Conn)) + p.Offset)) = []*x509.Certificate{cert}

	// Use reflection to set Hello and ServerHello since types are not easily nameable
	hType := reflect.TypeOf(verifier.HandshakeState.Hello).Elem()
	hVal := reflect.New(hType)
	hVal.Elem().FieldByName("Raw").SetBytes(helloRaw)
	reflect.ValueOf(&verifier.HandshakeState).Elem().FieldByName("Hello").Set(hVal)

	shType := reflect.TypeOf(verifier.HandshakeState.ServerHello).Elem()
	shVal := reflect.New(shType)
	shVal.Elem().FieldByName("Raw").SetBytes(serverHelloRaw)
	reflect.ValueOf(&verifier.HandshakeState).Elem().FieldByName("ServerHello").Set(shVal)

	err = verifier.VerifyPeerCertificate(nil, nil)
	require.NoError(t, err)
	require.True(t, verifier.verified)
}
