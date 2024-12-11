package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"net"
	"net/netip"
	"os"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/ntp"
	aTLS "github.com/sagernet/sing/common/tls"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

var _ ConfigCompat = (*STDClientConfig)(nil)

type STDClientConfig struct {
	config *tls.Config
}

func (s *STDClientConfig) ServerName() string {
	return s.config.ServerName
}

func (s *STDClientConfig) SetServerName(serverName string) {
	s.config.ServerName = serverName
}

func (s *STDClientConfig) NextProtos() []string {
	return s.config.NextProtos
}

func (s *STDClientConfig) SetNextProtos(nextProto []string) {
	s.config.NextProtos = nextProto
}

func (s *STDClientConfig) Config() (*STDConfig, error) {
	return s.config, nil
}

func (s *STDClientConfig) Client(conn net.Conn) (Conn, error) {
	return tls.Client(conn, s.config), nil
}

func (s *STDClientConfig) Clone() Config {
	return &STDClientConfig{s.config.Clone()}
}

type STDECHClientConfig struct {
	STDClientConfig
}

func (s *STDClientConfig) ClientHandshake(ctx context.Context, conn net.Conn) (aTLS.Conn, error) {
	if len(s.config.EncryptedClientHelloConfigList) == 0 {
		message := &mDNS.Msg{
			MsgHdr: mDNS.MsgHdr{
				RecursionDesired: true,
			},
			Question: []mDNS.Question{
				{
					Name:   mDNS.Fqdn(s.config.ServerName),
					Qtype:  mDNS.TypeHTTPS,
					Qclass: mDNS.ClassINET,
				},
			},
		}
		dnsRouter := service.FromContext[adapter.Router](ctx)
		response, err := dnsRouter.Exchange(ctx, message)
		if err != nil {
			return nil, E.Cause(err, "fetch ECH config list")
		}
		if response.Rcode != mDNS.RcodeSuccess {
			return nil, E.Cause(dns.RCodeError(response.Rcode), "fetch ECH config list")
		}
		for _, rr := range response.Answer {
			switch resource := rr.(type) {
			case *mDNS.HTTPS:
				for _, value := range resource.Value {
					if value.Key().String() == "ech" {
						echConfigList, err := base64.StdEncoding.DecodeString(value.String())
						if err != nil {
							return nil, E.Cause(err, "decode ECH config")
						}
						s.config.EncryptedClientHelloConfigList = echConfigList
					}
				}
			}
		}
		return nil, E.New("no ECH config found in DNS records")
	}
	tlsConn, err := s.Client(conn)
	if err != nil {
		return nil, err
	}
	err = tlsConn.HandshakeContext(ctx)
	if err != nil {
		return nil, err
	}
	return tlsConn, nil
}

func (s *STDECHClientConfig) Clone() Config {
	return &STDECHClientConfig{STDClientConfig{s.config.Clone()}}
}

func NewSTDClient(ctx context.Context, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
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

	var tlsConfig tls.Config
	tlsConfig.Time = ntp.TimeFuncFromContext(ctx)
	if options.DisableSNI {
		tlsConfig.ServerName = "127.0.0.1"
	} else {
		tlsConfig.ServerName = serverName
	}
	if options.Insecure {
		tlsConfig.InsecureSkipVerify = options.Insecure
	} else if options.DisableSNI {
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyConnection = func(state tls.ConnectionState) error {
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
	if options.ECH != nil && options.ECH.Enabled {
		var echConfig []byte
		if len(options.ECH.Config) > 0 {
			echConfig = []byte(strings.Join(options.ECH.Config, "\n"))
		} else if options.ECH.ConfigPath != "" {
			content, err := os.ReadFile(options.ECH.ConfigPath)
			if err != nil {
				return nil, E.Cause(err, "read ECH config")
			}
			echConfig = content
		}
		if echConfig != nil {
			tlsConfig.EncryptedClientHelloConfigList = echConfig
		}
		return &STDECHClientConfig{STDClientConfig{&tlsConfig}}, nil
	}
	return &STDClientConfig{&tlsConfig}, nil
}
