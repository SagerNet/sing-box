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

	utls "github.com/refraction-networking/utls"
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

func (e *UTLSClientConfig) Client(conn net.Conn) Conn {
	return &utlsConnWrapper{utls.UClient(conn, e.config.Clone(), e.id)}
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

func (e *UTLSClientConfig) Clone() Config {
	return &UTLSClientConfig{
		config: e.config.Clone(),
		id:     e.id,
	}
}

func NewUTLSClient(router adapter.Router, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
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
	var id utls.ClientHelloID
	switch options.UTLS.Fingerprint {
	case "chrome", "":
		id = utls.HelloChrome_Auto
	case "firefox":
		id = utls.HelloFirefox_Auto
	case "edge":
		id = utls.HelloEdge_Auto
	case "safari":
		id = utls.HelloSafari_Auto
	case "360":
		id = utls.Hello360_Auto
	case "qq":
		id = utls.HelloQQ_Auto
	case "ios":
		id = utls.HelloIOS_Auto
	case "android":
		id = utls.HelloAndroid_11_OkHttp
	case "random":
		id = utls.HelloRandomized
	default:
		return nil, E.New("unknown uTLS fingerprint: ", options.UTLS.Fingerprint)
	}
	return &UTLSClientConfig{&tlsConfig, id}, nil
}
