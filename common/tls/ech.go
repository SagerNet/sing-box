//go:build go1.24

package tls

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	aTLS "github.com/sagernet/sing/common/tls"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/filemanager"

	mDNS "github.com/miekg/dns"
	"golang.org/x/crypto/cryptobyte"
)

func parseECHClientConfig(ctx context.Context, clientConfig ECHCapableConfig, options option.OutboundTLSOptions, domainResolver *option.DomainResolveOptions, directResolver bool) (Config, error) {
	var echConfig []byte
	if len(options.ECH.Config) > 0 {
		echConfig = []byte(strings.Join(options.ECH.Config, "\n"))
	} else if options.ECH.ConfigPath != "" {
		content, err := filemanager.ReadFile(ctx, options.ECH.ConfigPath)
		if err != nil {
			return nil, E.Cause(err, "read ECH config")
		}
		echConfig = content
	}
	//nolint:staticcheck
	if options.ECH.PQSignatureSchemesEnabled || options.ECH.DynamicRecordSizingDisabled {
		return nil, E.New("legacy ECH options are deprecated in sing-box 1.12.0 and removed in sing-box 1.13.0")
	}
	if len(echConfig) > 0 {
		block, rest := pem.Decode(echConfig)
		if block == nil || block.Type != "ECH CONFIGS" || len(rest) > 0 {
			return nil, E.New("invalid ECH configs pem")
		}
		clientConfig.SetECHConfigList(block.Bytes)
		return clientConfig, nil
	} else {
		if directResolver && (domainResolver == nil || domainResolver.Server == "") {
			return nil, E.New("missing domain resolver for ECH config query")
		}
		return &ECHClientConfig{
			ECHCapableConfig: clientConfig,
			ctx:              ctx,
			dnsRouter:        service.FromContext[adapter.DNSRouter](ctx),
			domainResolver:   domainResolver,
			queryServerName:  options.ECH.QueryServerName,
			state: &echClientState{
				entries: make(map[string]echClientEntry),
			},
		}, nil
	}
}

func parseECHServerConfig(ctx context.Context, options option.InboundTLSOptions, tlsConfig *tls.Config, echKeyPath *string) error {
	var echKey []byte
	if len(options.ECH.Key) > 0 {
		echKey = []byte(strings.Join(options.ECH.Key, "\n"))
	} else if options.ECH.KeyPath != "" {
		content, err := filemanager.ReadFile(ctx, options.ECH.KeyPath)
		if err != nil {
			return E.Cause(err, "read ECH keys")
		}
		echKey = content
		*echKeyPath = options.ECH.KeyPath
	} else {
		return E.New("missing ECH keys")
	}
	echKeys, err := parseECHKeys(echKey)
	if err != nil {
		return E.Cause(err, "parse ECH keys")
	}
	tlsConfig.EncryptedClientHelloKeys = echKeys
	//nolint:staticcheck
	if options.ECH.PQSignatureSchemesEnabled || options.ECH.DynamicRecordSizingDisabled {
		return E.New("legacy ECH options are deprecated in sing-box 1.12.0 and removed in sing-box 1.13.0")
	}
	return nil
}

func (c *STDServerConfig) setECHServerConfig(echKey []byte) error {
	echKeys, err := parseECHKeys(echKey)
	if err != nil {
		return err
	}
	c.access.Lock()
	config := c.config.Clone()
	config.EncryptedClientHelloKeys = echKeys
	c.config = config
	c.access.Unlock()
	return nil
}

func parseECHKeys(echKey []byte) ([]tls.EncryptedClientHelloKey, error) {
	block, _ := pem.Decode(echKey)
	if block == nil || block.Type != "ECH KEYS" {
		return nil, E.New("invalid ECH keys pem")
	}
	echKeys, err := UnmarshalECHKeys(block.Bytes)
	if err != nil {
		return nil, E.Cause(err, "parse ECH keys")
	}
	return echKeys, nil
}

type echClientState struct {
	access  sync.Mutex
	entries map[string]echClientEntry
}

type echClientEntry struct {
	configList []byte
	expireAt   time.Time
}

type ECHClientConfig struct {
	ECHCapableConfig
	ctx             context.Context
	dnsRouter       adapter.DNSRouter
	domainResolver  *option.DomainResolveOptions
	queryServerName string
	state           *echClientState
}

func (s *ECHClientConfig) ClientHandshake(ctx context.Context, conn net.Conn) (aTLS.Conn, error) {
	tlsConfig, err := s.handshakeConfig(ctx)
	if err != nil {
		return nil, err
	}
	tlsConn, err := tlsConfig.Client(conn)
	if err != nil {
		return nil, err
	}
	err = tlsConn.HandshakeContext(ctx)
	if err != nil {
		return nil, err
	}
	return tlsConn, nil
}

func (s *ECHClientConfig) handshakeConfig(ctx context.Context) (ECHCapableConfig, error) {
	echConfigList, err := s.loadECHConfigList(ctx)
	if err != nil {
		return nil, err
	}
	tlsConfig := s.ECHCapableConfig.Clone().(ECHCapableConfig)
	tlsConfig.SetECHConfigList(echConfigList)
	return tlsConfig, nil
}

func (s *ECHClientConfig) echQueryServerName() string {
	if s.queryServerName != "" {
		return s.queryServerName
	}
	return s.ServerName()
}

func (s *ECHClientConfig) loadECHConfigList(ctx context.Context) ([]byte, error) {
	queryServerName := s.echQueryServerName()
	s.state.access.Lock()
	cachedEntry := s.state.entries[queryServerName]
	s.state.access.Unlock()
	if time.Now().Before(cachedEntry.expireAt) {
		return cachedEntry.configList, nil
	}
	queryOptions, err := dialer.NewDNSQueryOptions(s.ctx, s.domainResolver, true)
	if err != nil {
		if s.domainResolver == nil || s.domainResolver.Server == "" {
			return nil, E.Cause(err, "fetch ECH config list for ", queryServerName, ": ECH config query requires `domain_resolver` or `route.default_domain_resolver`")
		}
		return nil, E.Cause(err, "fetch ECH config list for ", queryServerName)
	}
	message := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			RecursionDesired: true,
		},
		Question: []mDNS.Question{
			{
				Name:   mDNS.Fqdn(queryServerName),
				Qtype:  mDNS.TypeHTTPS,
				Qclass: mDNS.ClassINET,
			},
		},
	}
	response, err := s.dnsRouter.Exchange(ctx, message, queryOptions)
	if err != nil {
		return nil, E.Cause(err, "fetch ECH config list for ", queryServerName)
	}
	if response.Rcode != mDNS.RcodeSuccess {
		return nil, E.Cause(dns.RcodeError(response.Rcode), "fetch ECH config list for ", queryServerName)
	}
	var (
		echConfigList []byte
		timeToLive    time.Duration
	)
match:
	for _, rr := range response.Answer {
		record, isHTTPS := rr.(*mDNS.HTTPS)
		if !isHTTPS {
			continue
		}
		for _, value := range record.Value {
			echConfig, isECHConfig := value.(*mDNS.SVCBECHConfig)
			if isECHConfig && len(echConfig.ECH) > 0 {
				echConfigList = echConfig.ECH
				timeToLive = time.Duration(rr.Header().Ttl) * time.Second
				break match
			}
		}
	}
	if len(echConfigList) == 0 {
		return nil, E.New("no ECH config found in DNS records for ", queryServerName)
	}
	s.state.access.Lock()
	defer s.state.access.Unlock()
	currentEntry := s.state.entries[queryServerName]
	if currentEntry.expireAt.Equal(cachedEntry.expireAt) {
		currentEntry = echClientEntry{
			configList: echConfigList,
			expireAt:   time.Now().Add(timeToLive),
		}
		s.state.entries[queryServerName] = currentEntry
	}
	return currentEntry.configList, nil
}

func (s *ECHClientConfig) ECHConfigList() []byte {
	s.state.access.Lock()
	defer s.state.access.Unlock()
	return s.state.entries[s.echQueryServerName()].configList
}

func (s *ECHClientConfig) SetECHConfigList(echConfigList []byte) {
	queryServerName := s.echQueryServerName()
	s.state.access.Lock()
	defer s.state.access.Unlock()
	s.state.entries[queryServerName] = echClientEntry{
		configList: echConfigList,
		expireAt:   time.Now().Add(C.DefaultDNSTTL * time.Second),
	}
}

func (s *ECHClientConfig) Clone() Config {
	return &ECHClientConfig{
		ECHCapableConfig: s.ECHCapableConfig.Clone().(ECHCapableConfig),
		ctx:              s.ctx,
		dnsRouter:        s.dnsRouter,
		domainResolver:   s.domainResolver,
		queryServerName:  s.queryServerName,
		state:            s.state,
	}
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
