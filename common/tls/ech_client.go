//go:build with_ech

package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"net"
	"net/netip"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	cftls "github.com/sagernet/sing-box/transport/cloudflaretls"
	"github.com/sagernet/sing-dns"
	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
)

type echClientConfig struct {
	config *cftls.Config
}

func (e *echClientConfig) NextProtos() []string {
	return e.config.NextProtos
}

func (e *echClientConfig) SetNextProtos(nextProto []string) {
	e.config.NextProtos = nextProto
}

func (e *echClientConfig) Config() (*STDConfig, error) {
	return nil, E.New("unsupported usage for ECH")
}

func (e *echClientConfig) Client(conn net.Conn) Conn {
	return &echConnWrapper{cftls.Client(conn, e.config)}
}

type echConnWrapper struct {
	*cftls.Conn
}

func (c *echConnWrapper) ConnectionState() tls.ConnectionState {
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

func newECHClient(router adapter.Router, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
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

	var tlsConfig cftls.Config
	if options.DisableSNI {
		tlsConfig.ServerName = "127.0.0.1"
	} else {
		tlsConfig.ServerName = serverName
	}
	if options.Insecure {
		tlsConfig.InsecureSkipVerify = options.Insecure
	} else if options.DisableSNI {
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyConnection = func(state cftls.ConnectionState) error {
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
			for _, tlsCipherSuite := range cftls.CipherSuites() {
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

	// ECH Config

	tlsConfig.ECHEnabled = true
	tlsConfig.PQSignatureSchemesEnabled = options.ECH.PQSignatureSchemesEnabled
	tlsConfig.DynamicRecordSizingDisabled = options.ECH.DynamicRecordSizingDisabled
	if options.ECH.Config != "" {
		clientConfigContent, err := base64.StdEncoding.DecodeString(options.ECH.Config)
		if err != nil {
			return nil, err
		}
		clientConfig, err := cftls.UnmarshalECHConfigs(clientConfigContent)
		if err != nil {
			return nil, err
		}
		tlsConfig.ClientECHConfigs = clientConfig
	} else {
		tlsConfig.GetClientECHConfigs = fetchECHClientConfig(router)
	}
	return &echClientConfig{&tlsConfig}, nil
}

func fetchECHClientConfig(router adapter.Router) func(ctx context.Context, serverName string) ([]cftls.ECHConfig, error) {
	return func(ctx context.Context, serverName string) ([]cftls.ECHConfig, error) {
		message := &mDNS.Msg{
			MsgHdr: mDNS.MsgHdr{
				RecursionDesired: true,
			},
			Question: []mDNS.Question{
				{
					Name:   serverName + ".",
					Qtype:  mDNS.TypeHTTPS,
					Qclass: mDNS.ClassINET,
				},
			},
		}
		response, err := router.Exchange(ctx, message)
		if err != nil {
			return nil, err
		}
		if response.Rcode != mDNS.RcodeSuccess {
			return nil, dns.RCodeError(response.Rcode)
		}
		for _, rr := range response.Answer {
			switch resource := rr.(type) {
			case *mDNS.HTTPS:
				for _, value := range resource.Value {
					if value.Key().String() == "ech" {
						echConfig, err := base64.StdEncoding.DecodeString(value.String())
						if err != nil {
							return nil, E.Cause(err, "decode ECH config")
						}
						return cftls.UnmarshalECHConfigs(echConfig)
					}
				}
			default:
				return nil, E.New("unknown resource record type: ", resource.Header().Rrtype)
			}
		}
		return nil, E.New("no ECH config found")
	}
}
