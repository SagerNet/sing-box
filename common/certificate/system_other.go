//go:build !android || !cgo

package certificate

import "crypto/x509"

func systemCertificates() []*x509.Certificate {
	return nil
}
