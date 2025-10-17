package tls

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"net"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tlsfragment"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/ntp"
)

type STDClientConfig struct {
	ctx                   context.Context
	config                *tls.Config
	fragment              bool
	fragmentFallbackDelay time.Duration
	recordFragment        bool
}

func (c *STDClientConfig) ServerName() string {
	return c.config.ServerName
}

func (c *STDClientConfig) SetServerName(serverName string) {
	c.config.ServerName = serverName
}

func (c *STDClientConfig) NextProtos() []string {
	return c.config.NextProtos
}

func (c *STDClientConfig) SetNextProtos(nextProto []string) {
	c.config.NextProtos = nextProto
}

func (c *STDClientConfig) STDConfig() (*STDConfig, error) {
	return c.config, nil
}

func (c *STDClientConfig) Client(conn net.Conn) (Conn, error) {
	if c.recordFragment {
		conn = tf.NewConn(conn, c.ctx, c.fragment, c.recordFragment, c.fragmentFallbackDelay)
	}
	return tls.Client(conn, c.config), nil
}

func (c *STDClientConfig) Clone() Config {
	return &STDClientConfig{
		ctx:                   c.ctx,
		config:                c.config.Clone(),
		fragment:              c.fragment,
		fragmentFallbackDelay: c.fragmentFallbackDelay,
		recordFragment:        c.recordFragment,
	}
}

func (c *STDClientConfig) ECHConfigList() []byte {
	return c.config.EncryptedClientHelloConfigList
}

func (c *STDClientConfig) SetECHConfigList(EncryptedClientHelloConfigList []byte) {
	c.config.EncryptedClientHelloConfigList = EncryptedClientHelloConfigList
}

func NewSTDClient(ctx context.Context, logger logger.ContextLogger, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
	var serverName string
	if options.ServerName != "" {
		serverName = options.ServerName
	} else if serverAddress != "" {
		serverName = serverAddress
	}
	if serverName == "" && !options.Insecure {
		return nil, E.New("missing server_name or insecure=true")
	}

	var tlsConfig tls.Config
	tlsConfig.Time = ntp.TimeFuncFromContext(ctx)
	tlsConfig.RootCAs = adapter.RootPoolFromContext(ctx)
	if !options.DisableSNI {
		tlsConfig.ServerName = serverName
	}
	if options.Insecure {
		tlsConfig.InsecureSkipVerify = options.Insecure
	} else if options.DisableSNI {
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyConnection = func(state tls.ConnectionState) error {
			verifyOptions := x509.VerifyOptions{
				Roots:         tlsConfig.RootCAs,
				DNSName:       serverName,
				Intermediates: x509.NewCertPool(),
			}
			for _, cert := range state.PeerCertificates[1:] {
				verifyOptions.Intermediates.AddCert(cert)
			}
			if tlsConfig.Time != nil {
				verifyOptions.CurrentTime = tlsConfig.Time()
			}
			_, err := state.PeerCertificates[0].Verify(verifyOptions)
			return err
		}
	}
	if len(options.CertificatePublicKeySHA256) > 0 {
		if len(options.Certificate) > 0 || options.CertificatePath != "" {
			return nil, E.New("certificate_public_key_sha256 is conflict with certificate or certificate_path")
		}
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			return verifyPublicKeySHA256(options.CertificatePublicKeySHA256, rawCerts, tlsConfig.Time)
		}
	}
	if len(options.ALPN) > 0 {
		tlsConfig.NextProtos = options.ALPN
	}
	if options.MinVersion != "" {
		minVersion, err := ParseTLSVersion(options.MinVersion)
		if err != nil {
			return nil, E.Cause(err, "parse min_version")
		}
		tlsConfig.MinVersion = minVersion
	}
	if options.MaxVersion != "" {
		maxVersion, err := ParseTLSVersion(options.MaxVersion)
		if err != nil {
			return nil, E.Cause(err, "parse max_version")
		}
		tlsConfig.MaxVersion = maxVersion
	}
	if options.CipherSuites != nil {
	find:
		for _, cipherSuite := range options.CipherSuites {
			for _, tlsCipherSuite := range tls.CipherSuites() {
				if cipherSuite == tlsCipherSuite.Name {
					tlsConfig.CipherSuites = append(tlsConfig.CipherSuites, tlsCipherSuite.ID)
					continue find
				}
			}
			return nil, E.New("unknown cipher_suite: ", cipherSuite)
		}
	}
	for _, curve := range options.CurvePreferences {
		tlsConfig.CurvePreferences = append(tlsConfig.CurvePreferences, tls.CurveID(curve))
	}
	var certificate []byte
	if len(options.Certificate) > 0 {
		certificate = []byte(strings.Join(options.Certificate, "\n"))
	} else if options.CertificatePath != "" {
		content, err := os.ReadFile(options.CertificatePath)
		if err != nil {
			return nil, E.Cause(err, "read certificate")
		}
		certificate = content
	}
	if len(certificate) > 0 {
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(certificate) {
			return nil, E.New("failed to parse certificate:\n\n", certificate)
		}
		tlsConfig.RootCAs = certPool
	}
	var clientCertificate []byte
	if len(options.ClientCertificate) > 0 {
		clientCertificate = []byte(strings.Join(options.ClientCertificate, "\n"))
	} else if options.ClientCertificatePath != "" {
		content, err := os.ReadFile(options.ClientCertificatePath)
		if err != nil {
			return nil, E.Cause(err, "read client certificate")
		}
		clientCertificate = content
	}
	var clientKey []byte
	if len(options.ClientKey) > 0 {
		clientKey = []byte(strings.Join(options.ClientKey, "\n"))
	} else if options.ClientKeyPath != "" {
		content, err := os.ReadFile(options.ClientKeyPath)
		if err != nil {
			return nil, E.Cause(err, "read client key")
		}
		clientKey = content
	}
	if len(clientCertificate) > 0 && len(clientKey) > 0 {
		keyPair, err := tls.X509KeyPair(clientCertificate, clientKey)
		if err != nil {
			return nil, E.Cause(err, "parse client x509 key pair")
		}
		tlsConfig.Certificates = []tls.Certificate{keyPair}
	} else if len(clientCertificate) > 0 || len(clientKey) > 0 {
		return nil, E.New("client certificate and client key must be provided together")
	}
	var config Config = &STDClientConfig{ctx, &tlsConfig, options.Fragment, time.Duration(options.FragmentFallbackDelay), options.RecordFragment}
	if options.ECH != nil && options.ECH.Enabled {
		var err error
		config, err = parseECHClientConfig(ctx, config.(ECHCapableConfig), options)
		if err != nil {
			return nil, err
		}
	}
	if options.KernelRx || options.KernelTx {
		if !C.IsLinux {
			return nil, E.New("kTLS is only supported on Linux")
		}
		config = &KTLSClientConfig{
			Config:   config,
			logger:   logger,
			kernelTx: options.KernelTx,
			kernelRx: options.KernelRx,
		}
	}
	return config, nil
}

func verifyPublicKeySHA256(knownHashValues [][]byte, rawCerts [][]byte, timeFunc func() time.Time) error {
	leafCertificate, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		return E.Cause(err, "failed to parse leaf certificate")
	}

	pubKeyBytes, err := x509.MarshalPKIXPublicKey(leafCertificate.PublicKey)
	if err != nil {
		return E.Cause(err, "failed to marshal public key")
	}
	hashValue := sha256.Sum256(pubKeyBytes)
	for _, value := range knownHashValues {
		if bytes.Equal(value, hashValue[:]) {
			return nil
		}
	}
	return E.New("unrecognized remote public key: ", base64.StdEncoding.EncodeToString(hashValue[:]))
}
