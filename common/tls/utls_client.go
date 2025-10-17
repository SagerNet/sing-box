//go:build with_utls

package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tlsfragment"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/ntp"

	utls "github.com/metacubex/utls"
	"golang.org/x/net/http2"
)

type UTLSClientConfig struct {
	ctx                   context.Context
	config                *utls.Config
	id                    utls.ClientHelloID
	fragment              bool
	fragmentFallbackDelay time.Duration
	recordFragment        bool
}

func (c *UTLSClientConfig) ServerName() string {
	return c.config.ServerName
}

func (c *UTLSClientConfig) SetServerName(serverName string) {
	c.config.ServerName = serverName
}

func (c *UTLSClientConfig) NextProtos() []string {
	return c.config.NextProtos
}

func (c *UTLSClientConfig) SetNextProtos(nextProto []string) {
	if len(nextProto) == 1 && nextProto[0] == http2.NextProtoTLS {
		nextProto = append(nextProto, "http/1.1")
	}
	c.config.NextProtos = nextProto
}

func (c *UTLSClientConfig) STDConfig() (*STDConfig, error) {
	return nil, E.New("unsupported usage for uTLS")
}

func (c *UTLSClientConfig) Client(conn net.Conn) (Conn, error) {
	if c.recordFragment {
		conn = tf.NewConn(conn, c.ctx, c.fragment, c.recordFragment, c.fragmentFallbackDelay)
	}
	return &utlsALPNWrapper{utlsConnWrapper{utls.UClient(conn, c.config.Clone(), c.id)}, c.config.NextProtos}, nil
}

func (c *UTLSClientConfig) SetSessionIDGenerator(generator func(clientHello []byte, sessionID []byte) error) {
	c.config.SessionIDGenerator = generator
}

func (c *UTLSClientConfig) Clone() Config {
	return &UTLSClientConfig{
		c.ctx, c.config.Clone(), c.id, c.fragment, c.fragmentFallbackDelay, c.recordFragment,
	}
}

func (c *UTLSClientConfig) ECHConfigList() []byte {
	return c.config.EncryptedClientHelloConfigList
}

func (c *UTLSClientConfig) SetECHConfigList(EncryptedClientHelloConfigList []byte) {
	c.config.EncryptedClientHelloConfigList = EncryptedClientHelloConfigList
}

type utlsConnWrapper struct {
	*utls.UConn
}

func (c *utlsConnWrapper) ConnectionState() tls.ConnectionState {
	state := c.Conn.ConnectionState()
	//nolint:staticcheck
	return tls.ConnectionState{
		Version:                     state.Version,
		HandshakeComplete:           state.HandshakeComplete,
		DidResume:                   state.DidResume,
		CipherSuite:                 state.CipherSuite,
		NegotiatedProtocol:          state.NegotiatedProtocol,
		NegotiatedProtocolIsMutual:  state.NegotiatedProtocolIsMutual,
		ServerName:                  state.ServerName,
		PeerCertificates:            state.PeerCertificates,
		VerifiedChains:              state.VerifiedChains,
		SignedCertificateTimestamps: state.SignedCertificateTimestamps,
		OCSPResponse:                state.OCSPResponse,
		TLSUnique:                   state.TLSUnique,
	}
}

func (c *utlsConnWrapper) Upstream() any {
	return c.UConn
}

func (c *utlsConnWrapper) ReaderReplaceable() bool {
	return true
}

func (c *utlsConnWrapper) WriterReplaceable() bool {
	return true
}

type utlsALPNWrapper struct {
	utlsConnWrapper
	nextProtocols []string
}

func (c *utlsALPNWrapper) HandshakeContext(ctx context.Context) error {
	if len(c.nextProtocols) > 0 {
		err := c.BuildHandshakeState()
		if err != nil {
			return err
		}
		for _, extension := range c.Extensions {
			if alpnExtension, isALPN := extension.(*utls.ALPNExtension); isALPN {
				alpnExtension.AlpnProtocols = c.nextProtocols
				err = c.BuildHandshakeState()
				if err != nil {
					return err
				}
				break
			}
		}
	}
	return c.UConn.HandshakeContext(ctx)
}

func NewUTLSClient(ctx context.Context, logger logger.ContextLogger, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
	var serverName string
	if options.ServerName != "" {
		serverName = options.ServerName
	} else if serverAddress != "" {
		serverName = serverAddress
	}
	if serverName == "" && !options.Insecure {
		return nil, E.New("missing server_name or insecure=true")
	}

	var tlsConfig utls.Config
	tlsConfig.Time = ntp.TimeFuncFromContext(ctx)
	tlsConfig.RootCAs = adapter.RootPoolFromContext(ctx)
	if !options.DisableSNI {
		tlsConfig.ServerName = serverName
	}
	if options.Insecure {
		tlsConfig.InsecureSkipVerify = options.Insecure
	} else if options.DisableSNI {
		if options.Reality != nil && options.Reality.Enabled {
			return nil, E.New("disable_sni is unsupported in reality")
		}
		tlsConfig.InsecureServerNameToVerify = serverName
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
		keyPair, err := utls.X509KeyPair(clientCertificate, clientKey)
		if err != nil {
			return nil, E.Cause(err, "parse client x509 key pair")
		}
		tlsConfig.Certificates = []utls.Certificate{keyPair}
	} else if len(clientCertificate) > 0 || len(clientKey) > 0 {
		return nil, E.New("client certificate and client key must be provided together")
	}
	id, err := uTLSClientHelloID(options.UTLS.Fingerprint)
	if err != nil {
		return nil, err
	}
	var config Config = &UTLSClientConfig{ctx, &tlsConfig, id, options.Fragment, time.Duration(options.FragmentFallbackDelay), options.RecordFragment}
	if options.ECH != nil && options.ECH.Enabled {
		if options.Reality != nil && options.Reality.Enabled {
			return nil, E.New("Reality is conflict with ECH")
		}
		config, err = parseECHClientConfig(ctx, config.(ECHCapableConfig), options)
		if err != nil {
			return nil, err
		}
	}
	if (options.KernelRx || options.KernelTx) && !common.PtrValueOrDefault(options.Reality).Enabled {
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

var (
	randomFingerprint     utls.ClientHelloID
	randomizedFingerprint utls.ClientHelloID
)

func init() {
	modernFingerprints := []utls.ClientHelloID{
		utls.HelloChrome_Auto,
		utls.HelloFirefox_Auto,
		utls.HelloEdge_Auto,
		utls.HelloSafari_Auto,
		utls.HelloIOS_Auto,
	}
	randomFingerprint = modernFingerprints[rand.Intn(len(modernFingerprints))]

	weights := utls.DefaultWeights
	weights.TLSVersMax_Set_VersionTLS13 = 1
	weights.FirstKeyShare_Set_CurveP256 = 0
	randomizedFingerprint = utls.HelloRandomized
	randomizedFingerprint.Seed, _ = utls.NewPRNGSeed()
	randomizedFingerprint.Weights = &weights
}

func uTLSClientHelloID(name string) (utls.ClientHelloID, error) {
	switch name {
	case "chrome_psk", "chrome_psk_shuffle", "chrome_padding_psk_shuffle", "chrome_pq", "chrome_pq_psk":
		fallthrough
	case "chrome", "":
		return utls.HelloChrome_Auto, nil
	case "firefox":
		return utls.HelloFirefox_Auto, nil
	case "edge":
		return utls.HelloEdge_Auto, nil
	case "safari":
		return utls.HelloSafari_Auto, nil
	case "360":
		return utls.Hello360_Auto, nil
	case "qq":
		return utls.HelloQQ_Auto, nil
	case "ios":
		return utls.HelloIOS_Auto, nil
	case "android":
		return utls.HelloAndroid_11_OkHttp, nil
	case "random":
		return randomFingerprint, nil
	case "randomized":
		return randomizedFingerprint, nil
	default:
		return utls.ClientHelloID{}, E.New("unknown uTLS fingerprint: ", name)
	}
}
