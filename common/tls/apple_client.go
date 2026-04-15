//go:build darwin && cgo

package tls

import (
	"context"
	"net"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	boxConstant "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/ntp"
	"github.com/sagernet/sing/service"
)

type appleCertificateStore interface {
	StoreKind() string
	CurrentPEM() []string
}

type appleClientConfig struct {
	serverName                 string
	nextProtos                 []string
	handshakeTimeout           time.Duration
	minVersion                 uint16
	maxVersion                 uint16
	insecure                   bool
	anchorPEM                  string
	anchorOnly                 bool
	certificatePublicKeySHA256 [][]byte
	timeFunc                   func() time.Time
}

func (c *appleClientConfig) ServerName() string {
	return c.serverName
}

func (c *appleClientConfig) SetServerName(serverName string) {
	c.serverName = serverName
}

func (c *appleClientConfig) NextProtos() []string {
	return c.nextProtos
}

func (c *appleClientConfig) SetNextProtos(nextProto []string) {
	c.nextProtos = append(c.nextProtos[:0], nextProto...)
}

func (c *appleClientConfig) HandshakeTimeout() time.Duration {
	return c.handshakeTimeout
}

func (c *appleClientConfig) SetHandshakeTimeout(timeout time.Duration) {
	c.handshakeTimeout = timeout
}

func (c *appleClientConfig) STDConfig() (*STDConfig, error) {
	return nil, E.New("unsupported usage for Apple TLS engine")
}

func (c *appleClientConfig) Client(conn net.Conn) (Conn, error) {
	return nil, os.ErrInvalid
}

func (c *appleClientConfig) Clone() Config {
	return &appleClientConfig{
		serverName:                 c.serverName,
		nextProtos:                 append([]string(nil), c.nextProtos...),
		handshakeTimeout:           c.handshakeTimeout,
		minVersion:                 c.minVersion,
		maxVersion:                 c.maxVersion,
		insecure:                   c.insecure,
		anchorPEM:                  c.anchorPEM,
		anchorOnly:                 c.anchorOnly,
		certificatePublicKeySHA256: append([][]byte(nil), c.certificatePublicKeySHA256...),
		timeFunc:                   c.timeFunc,
	}
}

func newAppleClient(ctx context.Context, logger logger.ContextLogger, serverAddress string, options option.OutboundTLSOptions, allowEmptyServerName bool) (Config, error) {
	validated, err := ValidateAppleTLSOptions(ctx, options, "Apple TLS engine")
	if err != nil {
		return nil, err
	}

	var serverName string
	if options.ServerName != "" {
		serverName = options.ServerName
	} else if serverAddress != "" {
		serverName = serverAddress
	}
	if serverName == "" && !options.Insecure && !allowEmptyServerName {
		return nil, errMissingServerName
	}

	var handshakeTimeout time.Duration
	if options.HandshakeTimeout > 0 {
		handshakeTimeout = options.HandshakeTimeout.Build()
	} else {
		handshakeTimeout = boxConstant.TCPTimeout
	}

	return &appleClientConfig{
		serverName:                 serverName,
		nextProtos:                 append([]string(nil), options.ALPN...),
		handshakeTimeout:           handshakeTimeout,
		minVersion:                 validated.MinVersion,
		maxVersion:                 validated.MaxVersion,
		insecure:                   options.Insecure || len(options.CertificatePublicKeySHA256) > 0,
		anchorPEM:                  validated.AnchorPEM,
		anchorOnly:                 validated.AnchorOnly,
		certificatePublicKeySHA256: append([][]byte(nil), options.CertificatePublicKeySHA256...),
		timeFunc:                   ntp.TimeFuncFromContext(ctx),
	}, nil
}

type AppleTLSValidated struct {
	MinVersion uint16
	MaxVersion uint16
	AnchorPEM  string
	AnchorOnly bool
}

func ValidateAppleTLSOptions(ctx context.Context, options option.OutboundTLSOptions, engineName string) (AppleTLSValidated, error) {
	if options.Reality != nil && options.Reality.Enabled {
		return AppleTLSValidated{}, E.New("reality is unsupported in ", engineName)
	}
	if options.UTLS != nil && options.UTLS.Enabled {
		return AppleTLSValidated{}, E.New("utls is unsupported in ", engineName)
	}
	if options.ECH != nil && options.ECH.Enabled {
		return AppleTLSValidated{}, E.New("ech is unsupported in ", engineName)
	}
	if options.DisableSNI {
		return AppleTLSValidated{}, E.New("disable_sni is unsupported in ", engineName)
	}
	if len(options.CipherSuites) > 0 {
		return AppleTLSValidated{}, E.New("cipher_suites is unsupported in ", engineName)
	}
	if len(options.CurvePreferences) > 0 {
		return AppleTLSValidated{}, E.New("curve_preferences is unsupported in ", engineName)
	}
	if len(options.ClientCertificate) > 0 || options.ClientCertificatePath != "" || len(options.ClientKey) > 0 || options.ClientKeyPath != "" {
		return AppleTLSValidated{}, E.New("client certificate is unsupported in ", engineName)
	}
	if options.Fragment || options.RecordFragment {
		return AppleTLSValidated{}, E.New("tls fragment is unsupported in ", engineName)
	}
	if options.KernelTx || options.KernelRx {
		return AppleTLSValidated{}, E.New("ktls is unsupported in ", engineName)
	}
	if options.Spoof != "" || options.SpoofMethod != "" {
		return AppleTLSValidated{}, E.New("spoof is unsupported in ", engineName)
	}
	if len(options.CertificatePublicKeySHA256) > 0 && (len(options.Certificate) > 0 || options.CertificatePath != "") {
		return AppleTLSValidated{}, E.New("certificate_public_key_sha256 is conflict with certificate or certificate_path")
	}
	var minVersion uint16
	if options.MinVersion != "" {
		var err error
		minVersion, err = ParseTLSVersion(options.MinVersion)
		if err != nil {
			return AppleTLSValidated{}, E.Cause(err, "parse min_version")
		}
	}
	var maxVersion uint16
	if options.MaxVersion != "" {
		var err error
		maxVersion, err = ParseTLSVersion(options.MaxVersion)
		if err != nil {
			return AppleTLSValidated{}, E.Cause(err, "parse max_version")
		}
	}
	anchorPEM, anchorOnly, err := AppleAnchorPEM(ctx, options)
	if err != nil {
		return AppleTLSValidated{}, err
	}
	return AppleTLSValidated{
		MinVersion: minVersion,
		MaxVersion: maxVersion,
		AnchorPEM:  anchorPEM,
		AnchorOnly: anchorOnly,
	}, nil
}

func AppleAnchorPEM(ctx context.Context, options option.OutboundTLSOptions) (string, bool, error) {
	if len(options.Certificate) > 0 {
		return strings.Join(options.Certificate, "\n"), true, nil
	}
	if options.CertificatePath != "" {
		content, err := os.ReadFile(options.CertificatePath)
		if err != nil {
			return "", false, E.Cause(err, "read certificate")
		}
		return string(content), true, nil
	}

	certificateStore := service.FromContext[adapter.CertificateStore](ctx)
	if certificateStore == nil {
		return "", false, nil
	}
	store, ok := certificateStore.(appleCertificateStore)
	if !ok {
		return "", false, nil
	}

	switch store.StoreKind() {
	case boxConstant.CertificateStoreSystem, "":
		return strings.Join(store.CurrentPEM(), "\n"), false, nil
	case boxConstant.CertificateStoreMozilla, boxConstant.CertificateStoreChrome, boxConstant.CertificateStoreNone:
		return strings.Join(store.CurrentPEM(), "\n"), true, nil
	default:
		return "", false, E.New("unsupported certificate store for Apple TLS engine: ", store.StoreKind())
	}
}
