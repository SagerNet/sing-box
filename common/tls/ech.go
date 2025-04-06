//go:build go1.24

package tls

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/pem"
	"net"
	"os"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	aTLS "github.com/sagernet/sing/common/tls"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
	"golang.org/x/crypto/cryptobyte"
)

func parseECHClientConfig(ctx context.Context, options option.OutboundTLSOptions, tlsConfig *tls.Config) (Config, error) {
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
	//nolint:staticcheck
	if options.ECH.PQSignatureSchemesEnabled || options.ECH.DynamicRecordSizingDisabled {
		deprecated.Report(ctx, deprecated.OptionLegacyECHOptions)
	}
	if len(echConfig) > 0 {
		block, rest := pem.Decode(echConfig)
		if block == nil || block.Type != "ECH CONFIGS" || len(rest) > 0 {
			return nil, E.New("invalid ECH configs pem")
		}
		tlsConfig.EncryptedClientHelloConfigList = block.Bytes
		return &STDClientConfig{tlsConfig}, nil
	} else {
		return &STDECHClientConfig{STDClientConfig{tlsConfig}, service.FromContext[adapter.DNSRouter](ctx)}, nil
	}
}

func parseECHServerConfig(ctx context.Context, options option.InboundTLSOptions, tlsConfig *tls.Config, echKeyPath *string) error {
	var echKey []byte
	if len(options.ECH.Key) > 0 {
		echKey = []byte(strings.Join(options.ECH.Key, "\n"))
	} else if options.ECH.KeyPath != "" {
		content, err := os.ReadFile(options.ECH.KeyPath)
		if err != nil {
			return E.Cause(err, "read ECH keys")
		}
		echKey = content
		*echKeyPath = options.ECH.KeyPath
	} else {
		return E.New("missing ECH keys")
	}
	block, rest := pem.Decode(echKey)
	if block == nil || block.Type != "ECH KEYS" || len(rest) > 0 {
		return E.New("invalid ECH keys pem")
	}
	echKeys, err := UnmarshalECHKeys(block.Bytes)
	if err != nil {
		return E.Cause(err, "parse ECH keys")
	}
	tlsConfig.EncryptedClientHelloKeys = echKeys
	//nolint:staticcheck
	if options.ECH.PQSignatureSchemesEnabled || options.ECH.DynamicRecordSizingDisabled {
		deprecated.Report(ctx, deprecated.OptionLegacyECHOptions)
	}
	return nil
}

func reloadECHKeys(echKeyPath string, tlsConfig *tls.Config) error {
	echKey, err := os.ReadFile(echKeyPath)
	if err != nil {
		return E.Cause(err, "reload ECH keys from ", echKeyPath)
	}
	block, _ := pem.Decode(echKey)
	if block == nil || block.Type != "ECH KEYS" {
		return E.New("invalid ECH keys pem")
	}
	echKeys, err := UnmarshalECHKeys(block.Bytes)
	if err != nil {
		return E.Cause(err, "parse ECH keys")
	}
	tlsConfig.EncryptedClientHelloKeys = echKeys
	return nil
}

type STDECHClientConfig struct {
	STDClientConfig
	dnsRouter adapter.DNSRouter
}

func (s *STDECHClientConfig) ClientHandshake(ctx context.Context, conn net.Conn) (aTLS.Conn, error) {
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
		response, err := s.dnsRouter.Exchange(ctx, message, adapter.DNSQueryOptions{})
		if err != nil {
			return nil, E.Cause(err, "fetch ECH config list")
		}
		if response.Rcode != mDNS.RcodeSuccess {
			return nil, E.Cause(dns.RcodeError(response.Rcode), "fetch ECH config list")
		}
	match:
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
						break match
					}
				}
			}
		}
		if len(s.config.EncryptedClientHelloConfigList) == 0 {
			return nil, E.New("no ECH config found in DNS records")
		}
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
	return &STDECHClientConfig{STDClientConfig{s.config.Clone()}, s.dnsRouter}
}

func UnmarshalECHKeys(raw []byte) ([]tls.EncryptedClientHelloKey, error) {
	var keys []tls.EncryptedClientHelloKey
	rawString := cryptobyte.String(raw)
	for !rawString.Empty() {
		var key tls.EncryptedClientHelloKey
		if !rawString.ReadUint16LengthPrefixed((*cryptobyte.String)(&key.PrivateKey)) {
			return nil, E.New("error parsing private key")
		}
		if !rawString.ReadUint16LengthPrefixed((*cryptobyte.String)(&key.Config)) {
			return nil, E.New("error parsing config")
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, E.New("empty ECH keys")
	}
	return keys, nil
}
