//go:build linux || darwin

package tlsspoof

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// generateSelfSignedCert returns a TLS certificate valid for the given SAN.
func generateSelfSignedCert(t *testing.T, commonName string, sans ...string) tls.Certificate {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     sans,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return cert
}

// TestIntegrationConn_RealTLSHandshake drives a real crypto/tls ClientHello
// through the spoofer and asserts the on-wire fake packet carries the fake SNI
// while the server receives the real SNI. This exercises the full
// `tls.Client(wrapped, config).Handshake()` path rather than a static hex
// payload, matching what user-facing code hits.
func TestIntegrationConn_RealTLSHandshake(t *testing.T) {
	requireRoot(t)
	const realSNI = "real.test"
	const fakeSNI = "fake.test"

	serverCert := generateSelfSignedCert(t, realSNI, realSNI)
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{serverCert}}

	listener, err := tls.Listen("tcp4", "127.0.0.1:0", tlsConfig)
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	serverSNI := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		tlsConn := conn.(*tls.Conn)
		_ = tlsConn.SetDeadline(time.Now().Add(3 * time.Second))
		if handshakeErr := tlsConn.Handshake(); handshakeErr != nil {
			serverSNI <- "handshake-error:" + handshakeErr.Error()
			return
		}
		serverSNI <- tlsConn.ConnectionState().ServerName
		_, _ = io.Copy(io.Discard, conn)
	}()

	addr := listener.Addr().(*net.TCPAddr)
	serverPort := uint16(addr.Port)
	raw, err := net.Dial("tcp4", addr.String())
	require.NoError(t, err)
	t.Cleanup(func() { raw.Close() })

	wrapped, err := NewConn(raw, MethodWrongSequence, fakeSNI)
	require.NoError(t, err)

	clientConfig := &tls.Config{
		ServerName:         realSNI,
		InsecureSkipVerify: true,
	}
	tlsClient := tls.Client(wrapped, clientConfig)
	t.Cleanup(func() { tlsClient.Close() })

	seen := tcpdumpObserverMulti(t, loopbackInterface, serverPort,
		[]string{realSNI, fakeSNI}, func() {
			_ = tlsClient.SetDeadline(time.Now().Add(3 * time.Second))
			err := tlsClient.Handshake()
			require.NoError(t, err, "TLS handshake must succeed (wrong-sequence fake is dropped by peer)")
		}, 4*time.Second)

	require.True(t, seen[realSNI],
		"real ClientHello on the wire must contain original SNI %q", realSNI)
	require.True(t, seen[fakeSNI],
		"fake ClientHello on the wire must contain fake SNI %q", fakeSNI)

	select {
	case sniOnServer := <-serverSNI:
		require.Equal(t, realSNI, sniOnServer,
			"TLS server must see the real SNI (fake packet dropped by peer TCP stack)")
	case <-time.After(3 * time.Second):
		t.Fatal("TLS server did not complete handshake")
	}
}
