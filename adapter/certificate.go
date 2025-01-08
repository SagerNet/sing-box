package adapter

import (
	"context"
	"crypto/x509"

	"github.com/sagernet/sing/service"
)

type CertificateStore interface {
	LifecycleService
	Pool() *x509.CertPool
}

func RootPoolFromContext(ctx context.Context) *x509.CertPool {
	store := service.FromContext[CertificateStore](ctx)
	if store == nil {
		return nil
	}
	return store.Pool()
}
