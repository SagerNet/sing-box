package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"
)

func GenerateKeyPair(parent *x509.Certificate, parentKey any, timeFunc func() time.Time, serverName string) (*tls.Certificate, error) {
	if timeFunc == nil {
		timeFunc = time.Now
	}
	privateKeyPem, publicKeyPem, err := GenerateCertificate(parent, parentKey, timeFunc, serverName, timeFunc().Add(time.Hour))
	if err != nil {
		return nil, err
	}
	certificate, err := tls.X509KeyPair(publicKeyPem, privateKeyPem)
	if err != nil {
		return nil, err
	}
	return &certificate, err
}

func GenerateCertificate(parent *x509.Certificate, parentKey any, timeFunc func() time.Time, serverName string, expire time.Time) (privateKeyPem []byte, publicKeyPem []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
	}
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return
	}
	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		NotBefore:             timeFunc().Add(time.Hour * -1),
		NotAfter:              expire,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		Subject: pkix.Name{
			CommonName: serverName,
		},
		DNSNames: []string{serverName},
	}
	if parent == nil {
		parent = template
		parentKey = key
	}
	publicDer, err := x509.CreateCertificate(rand.Reader, template, parent, key.Public(), parentKey)
	if err != nil {
		return
	}
	privateDer, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return
	}
	publicKeyPem = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: publicDer})
	privateKeyPem = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDer})
	return
}
