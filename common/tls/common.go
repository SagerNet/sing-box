package tls

import (
	"crypto/x509"
	"os"

	E "github.com/sagernet/sing/common/exceptions"
)

const (
	VersionTLS10 = 0x0301
	VersionTLS11 = 0x0302
	VersionTLS12 = 0x0303
	VersionTLS13 = 0x0304

	// Deprecated: SSLv3 is cryptographically broken, and is no longer
	// supported by this package. See golang.org/issue/32716.
	VersionSSL30 = 0x0300
)

func loadCertAsBytes(certificate string, certificatePath string) ([]byte, error) {
	if certificate != "" {
		return []byte(certificate), nil
	} else if certificatePath != "" {
		content, err := os.ReadFile(certificatePath)
		if err != nil {
			return nil, E.Cause(err, "read certificate")
		}
		return content, nil
	}
	return nil, nil
}

func loadCertAsPool(certificate string, certificatePath string) (*x509.CertPool, error) {
	certBytes, err := loadCertAsBytes(certificate, certificatePath)
	if err != nil {
		return nil, err
	}
	if certBytes == nil {
		return nil, nil
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(certBytes) {
		return nil, E.New("failed to parse certificate:\n\n", certBytes)
	}
	return certPool, nil
}
