//go:build with_utls

package tls

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/netip"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	utls "github.com/sagernet/utls"
)

type UTLSClientConfig struct {
	config *utls.Config
	id     utls.ClientHelloID
}

func (e *UTLSClientConfig) ServerName() string {
	return e.config.ServerName
}

func (e *UTLSClientConfig) SetServerName(serverName string) {
	e.config.ServerName = serverName
}

func (e *UTLSClientConfig) NextProtos() []string {
	return e.config.NextProtos
}

func (e *UTLSClientConfig) SetNextProtos(nextProto []string) {
	e.config.NextProtos = nextProto
}

func (e *UTLSClientConfig) Config() (*STDConfig, error) {
	return nil, E.New("unsupported usage for uTLS")
}

func (e *UTLSClientConfig) Client(conn net.Conn) (Conn, error) {
	return &utlsConnWrapper{utls.UClient(conn, e.config.Clone(), e.id)}, nil
}

func (e *UTLSClientConfig) SetSessionIDGenerator(generator func(clientHello []byte, sessionID []byte) error) {
	e.config.SessionIDGenerator = generator
}

func (e *UTLSClientConfig) Clone() Config {
	return &UTLSClientConfig{
		config: e.config.Clone(),
		id:     e.id,
	}
}

type utlsConnWrapper struct {
	*utls.UConn
}

func (c *utlsConnWrapper) ConnectionState() tls.ConnectionState {
	state := c.Conn.ConnectionState()
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

func NewUTLSClient(router adapter.Router, serverAddress string, options option.OutboundTLSOptions) (*UTLSClientConfig, error) {
	var serverName string
	if options.ServerName != "" {
		serverName = options.ServerName
	} else if serverAddress != "" {
		if _, err := netip.ParseAddr(serverName); err != nil {
			serverName = serverAddress
		}
	}
	if serverName == "" && !options.Insecure {
		return nil, E.New("missing server_name or insecure=true")
	}

	var tlsConfig utls.Config
	tlsConfig.Time = router.TimeFunc()
	if options.DisableSNI {
		tlsConfig.ServerName = "127.0.0.1"
	} else {
		tlsConfig.ServerName = serverName
	}
	if options.Insecure {
		tlsConfig.InsecureSkipVerify = options.Insecure
	} else if options.DisableSNI {
		return nil, E.New("disable_sni is unsupported in uTLS")
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
	if options.Certificate != "" {
		certificate = []byte(options.Certificate)
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
	id, err := uTLSClientHelloID(options.UTLS.Fingerprint)
	if err != nil {
		return nil, err
	}
	return &UTLSClientConfig{&tlsConfig, id}, nil
}

func uTLSClientHelloID(name string) (utls.ClientHelloID, error) {
	switch name {
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
		return utls.HelloRandomized, nil
	default:
		return utls.ClientHelloID{}, E.New("unknown uTLS fingerprint: ", name)
	}
}
