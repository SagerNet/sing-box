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

func GenerateKeyPair(timeFunc func() time.Time, serverName string) (*tls.Certificate, error) {
	if timeFunc == nil {
		timeFunc = time.Now
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		NotBefore:             timeFunc().Add(time.Hour * -1),
		NotAfter:              timeFunc().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		Subject: pkix.Name{
			CommonName: serverName,
		},
		DNSNames: []string{serverName},
	}
	publicDer, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		return nil, err
	}
	privateDer, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	publicPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: publicDer})
	privPem := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDer})
	keyPair, err := tls.X509KeyPair(publicPem, privPem)
	if err != nil {
		return nil, err
	}
	return &keyPair, err
}
