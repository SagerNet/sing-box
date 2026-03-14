package adapter

import (
	"crypto/tls"
)

type CertificateProvider interface {
	GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error)
}

type ACMECertificateProvider interface {
	CertificateProvider
	GetACMENextProtos() []string
}
