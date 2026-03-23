package adapter

import (
	"context"
	"crypto/tls"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

type CertificateProvider interface {
	GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error)
}

type ACMECertificateProvider interface {
	CertificateProvider
	GetACMENextProtos() []string
}

type CertificateProviderService interface {
	Lifecycle
	Type() string
	Tag() string
	CertificateProvider
}

type CertificateProviderRegistry interface {
	option.CertificateProviderOptionsRegistry
	Create(ctx context.Context, logger log.ContextLogger, tag string, providerType string, options any) (CertificateProviderService, error)
}

type CertificateProviderManager interface {
	Lifecycle
	CertificateProviders() []CertificateProviderService
	Get(tag string) (CertificateProviderService, bool)
	Remove(tag string) error
	Create(ctx context.Context, logger log.ContextLogger, tag string, providerType string, options any) error
}
