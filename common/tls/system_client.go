package tls

import (
	"context"
	"crypto/x509"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"
)

type SystemTLSValidated struct {
	MinVersion uint16
	MaxVersion uint16
	UserPEM    []byte
	Exclusive  bool
	Store      adapter.CertificateStore
}

func ValidateSystemTLSOptions(ctx context.Context, options option.OutboundTLSOptions, engineName string) (SystemTLSValidated, error) {
	if options.Reality != nil && options.Reality.Enabled {
		return SystemTLSValidated{}, E.New("reality is unsupported in ", engineName)
	}
	if options.UTLS != nil && options.UTLS.Enabled {
		return SystemTLSValidated{}, E.New("utls is unsupported in ", engineName)
	}
	if options.ECH != nil && options.ECH.Enabled {
		return SystemTLSValidated{}, E.New("ech is unsupported in ", engineName)
	}
	if options.DisableSNI {
		return SystemTLSValidated{}, E.New("disable_sni is unsupported in ", engineName)
	}
	if len(options.CipherSuites) > 0 {
		return SystemTLSValidated{}, E.New("cipher_suites is unsupported in ", engineName)
	}
	if len(options.CurvePreferences) > 0 {
		return SystemTLSValidated{}, E.New("curve_preferences is unsupported in ", engineName)
	}
	if len(options.ClientCertificate) > 0 || options.ClientCertificatePath != "" || len(options.ClientKey) > 0 || options.ClientKeyPath != "" {
		return SystemTLSValidated{}, E.New("client certificate is unsupported in ", engineName)
	}
	if options.Fragment || options.RecordFragment {
		return SystemTLSValidated{}, E.New("tls fragment is unsupported in ", engineName)
	}
	if options.KernelTx || options.KernelRx {
		return SystemTLSValidated{}, E.New("ktls is unsupported in ", engineName)
	}
	if options.Spoof != "" || options.SpoofMethod != "" {
		return SystemTLSValidated{}, E.New("spoof is unsupported in ", engineName)
	}
	if len(options.CertificatePublicKeySHA256) > 0 && (len(options.Certificate) > 0 || options.CertificatePath != "") {
		return SystemTLSValidated{}, E.New("certificate_public_key_sha256 is conflict with certificate or certificate_path")
	}
	var minVersion uint16
	if options.MinVersion != "" {
		parsed, err := ParseTLSVersion(options.MinVersion)
		if err != nil {
			return SystemTLSValidated{}, E.Cause(err, "parse min_version")
		}
		minVersion = parsed
	}
	var maxVersion uint16
	if options.MaxVersion != "" {
		parsed, err := ParseTLSVersion(options.MaxVersion)
		if err != nil {
			return SystemTLSValidated{}, E.Cause(err, "parse max_version")
		}
		maxVersion = parsed
	}
	userPEM, exclusive, store, err := resolveSystemAnchors(ctx, options)
	if err != nil {
		return SystemTLSValidated{}, err
	}
	return SystemTLSValidated{
		MinVersion: minVersion,
		MaxVersion: maxVersion,
		UserPEM:    userPEM,
		Exclusive:  exclusive,
		Store:      store,
	}, nil
}

func resolveSystemAnchors(ctx context.Context, options option.OutboundTLSOptions) ([]byte, bool, adapter.CertificateStore, error) {
	if len(options.Certificate) > 0 {
		return []byte(strings.Join(options.Certificate, "\n")), true, nil, nil
	}
	if options.CertificatePath != "" {
		content, err := os.ReadFile(options.CertificatePath)
		if err != nil {
			return nil, false, nil, E.Cause(err, "read certificate")
		}
		return content, true, nil, nil
	}
	store := service.FromContext[adapter.CertificateStore](ctx)
	if store == nil {
		return nil, false, nil, nil
	}
	return nil, store.ExclusiveAnchors(), store, nil
}

func verifySystemTLSPeer(roots *x509.CertPool, serverName string, timeFunc func() time.Time, peerCertificates []*x509.Certificate) error {
	if len(peerCertificates) == 0 {
		return E.New("no peer certificates")
	}
	intermediates := x509.NewCertPool()
	for _, cert := range peerCertificates[1:] {
		intermediates.AddCert(cert)
	}
	verifyOptions := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		DNSName:       serverName,
	}
	if timeFunc != nil {
		verifyOptions.CurrentTime = timeFunc()
	}
	_, err := peerCertificates[0].Verify(verifyOptions)
	return err
}
