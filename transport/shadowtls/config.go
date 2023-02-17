package shadowtls

import (
	"crypto/x509"
	"net"
	"net/netip"
	"os"

	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ tls.Config = (*ClientTLSConfig)(nil)

type ClientTLSConfig struct {
	config *sTLSConfig
}

func NewClientTLSConfig(serverAddress string, options option.OutboundTLSOptions, password string) (*ClientTLSConfig, error) {
	if options.ECH != nil && options.ECH.Enabled {
		return nil, E.New("ECH is not supported in shadowtls v3")
	} else if options.UTLS != nil && options.UTLS.Enabled {
		return nil, E.New("UTLS is not supported in shadowtls v3")
	}

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

	var tlsConfig sTLSConfig
	tlsConfig.SessionIDGenerator = generateSessionID(password)
	if options.DisableSNI {
		tlsConfig.ServerName = "127.0.0.1"
	} else {
		tlsConfig.ServerName = serverName
	}
	if options.Insecure {
		tlsConfig.InsecureSkipVerify = options.Insecure
	} else if options.DisableSNI {
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyConnection = func(state sTLSConnectionState) error {
			verifyOptions := x509.VerifyOptions{
				DNSName:       serverName,
				Intermediates: x509.NewCertPool(),
			}
			for _, cert := range state.PeerCertificates[1:] {
				verifyOptions.Intermediates.AddCert(cert)
			}
			_, err := state.PeerCertificates[0].Verify(verifyOptions)
			return err
		}
	}
	if len(options.ALPN) > 0 {
		tlsConfig.NextProtos = options.ALPN
	}
	if options.MinVersion != "" {
		minVersion, err := tls.ParseTLSVersion(options.MinVersion)
		if err != nil {
			return nil, E.Cause(err, "parse min_version")
		}
		tlsConfig.MinVersion = minVersion
	}
	if options.MaxVersion != "" {
		maxVersion, err := tls.ParseTLSVersion(options.MaxVersion)
		if err != nil {
			return nil, E.Cause(err, "parse max_version")
		}
		tlsConfig.MaxVersion = maxVersion
	}
	if options.CipherSuites != nil {
	find:
		for _, cipherSuite := range options.CipherSuites {
			for _, tlsCipherSuite := range sTLSCipherSuites() {
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
	return &ClientTLSConfig{&tlsConfig}, nil
}

func (c *ClientTLSConfig) ServerName() string {
	return c.config.ServerName
}

func (c *ClientTLSConfig) SetServerName(serverName string) {
	c.config.ServerName = serverName
}

func (c *ClientTLSConfig) NextProtos() []string {
	return c.config.NextProtos
}

func (c *ClientTLSConfig) SetNextProtos(nextProto []string) {
	c.config.NextProtos = nextProto
}

func (c *ClientTLSConfig) Config() (*tls.STDConfig, error) {
	return nil, E.New("unsupported usage for ShadowTLS")
}

func (c *ClientTLSConfig) Client(conn net.Conn) tls.Conn {
	return &shadowTLSConnWrapper{sTLSClient(conn, c.config)}
}

func (c *ClientTLSConfig) Clone() tls.Config {
	return &ClientTLSConfig{c.config.Clone()}
}

type shadowTLSConnWrapper struct {
	*sTLSConn
}

func (c *shadowTLSConnWrapper) ConnectionState() tls.ConnectionState {
	state := c.sTLSConn.ConnectionState()
	return tls.ConnectionState{
		Version:                     state.Version,
		HandshakeComplete:           state.HandshakeComplete,
		DidResume:                   state.DidResume,
		CipherSuite:                 state.CipherSuite,
		NegotiatedProtocol:          state.NegotiatedProtocol,
		ServerName:                  state.ServerName,
		PeerCertificates:            state.PeerCertificates,
		VerifiedChains:              state.VerifiedChains,
		SignedCertificateTimestamps: state.SignedCertificateTimestamps,
		OCSPResponse:                state.OCSPResponse,
	}
}
