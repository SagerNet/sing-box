package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sagernet/sing/common/rw"

	"github.com/stretchr/testify/require"
)

func createSelfSignedCertificate(t *testing.T, domain string) (caPem, certPem, keyPem string) {
	const userAndHostname = "sekai@nekohasekai.local"
	tempDir, err := os.MkdirTemp("", "sing-box-test")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})
	caKey, err := rsa.GenerateKey(rand.Reader, 3072)
	require.NoError(t, err)
	spkiASN1, err := x509.MarshalPKIXPublicKey(caKey.Public())
	var spki struct {
		Algorithm        pkix.AlgorithmIdentifier
		SubjectPublicKey asn1.BitString
	}
	_, err = asn1.Unmarshal(spkiASN1, &spki)
	require.NoError(t, err)
	skid := sha1.Sum(spki.SubjectPublicKey.Bytes)
	caTpl := &x509.Certificate{
		SerialNumber: randomSerialNumber(t),
		Subject: pkix.Name{
			Organization:       []string{"sing-box test CA"},
			OrganizationalUnit: []string{userAndHostname},
			CommonName:         "sing-box " + userAndHostname,
		},
		SubjectKeyId:          skid[:],
		NotAfter:              time.Now().AddDate(10, 0, 0),
		NotBefore:             time.Now(),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	caCert, err := x509.CreateCertificate(rand.Reader, caTpl, caTpl, caKey.Public(), caKey)
	require.NoError(t, err)
	err = rw.WriteFile(filepath.Join(tempDir, "ca.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert}))
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	domainTpl := &x509.Certificate{
		SerialNumber: randomSerialNumber(t),
		Subject: pkix.Name{
			Organization:       []string{"sing-box test certificate"},
			OrganizationalUnit: []string{"sing-box " + userAndHostname},
		},
		NotBefore: time.Now(), NotAfter: time.Now().AddDate(0, 0, 30),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	domainTpl.DNSNames = append(domainTpl.DNSNames, domain)
	cert, err := x509.CreateCertificate(rand.Reader, domainTpl, caTpl, key.Public(), caKey)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})
	err = rw.WriteFile(filepath.Join(tempDir, domain+".pem"), certPEM)
	require.NoError(t, err)
	err = rw.WriteFile(filepath.Join(tempDir, domain+".key.pem"), privPEM)
	require.NoError(t, err)
	return filepath.Join(tempDir, "ca.pem"), filepath.Join(tempDir, domain+".pem"), filepath.Join(tempDir, domain+".key.pem")
}

func randomSerialNumber(t *testing.T) *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)
	return serialNumber
}
