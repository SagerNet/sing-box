//go:build with_acme

package acme

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	boxtls "github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/caddyserver/certmagic"
	"github.com/caddyserver/zerossl"
	"github.com/libdns/acmedns"
	"github.com/libdns/alidns"
	"github.com/libdns/cloudflare"
	"github.com/mholt/acmez/v3/acme"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.ACMEServiceOptions](registry, C.TypeACME, NewService)
}

var _ adapter.ACMECertificateProvider = (*Service)(nil)

type Service struct {
	boxService.Adapter
	ctx        context.Context
	config     *certmagic.Config
	cache      *certmagic.Cache
	domain     []string
	nextProtos []string
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.ACMEServiceOptions) (adapter.Service, error) {
	acmeServer, err := resolveACMEServer(options.Provider)
	if err != nil {
		return nil, err
	}
	if options.TestCA != "" && !strings.HasPrefix(options.TestCA, "https://") {
		return nil, E.New("unsupported ACME test CA: ", options.TestCA)
	}
	if options.ACMETimeout < 0 {
		return nil, E.New("invalid ACME timeout: ", options.ACMETimeout)
	}
	if options.CertificateLifetime < 0 {
		return nil, E.New("invalid certificate lifetime: ", options.CertificateLifetime)
	}
	if options.RenewalWindowRatio < 0 || options.RenewalWindowRatio >= 1 {
		return nil, E.New("renewal_window_ratio must be in [0, 1)")
	}
	if acmeServer == certmagic.ZeroSSLProductionCA &&
		(options.ExternalAccount == nil || options.ExternalAccount.KeyID == "") &&
		strings.TrimSpace(options.Email) == "" &&
		strings.TrimSpace(options.AccountKey) == "" {
		return nil, E.New("email is required to use the ZeroSSL ACME endpoint without external_account or account_key")
	}

	var storage certmagic.Storage
	if options.DataDirectory != "" {
		storage = &certmagic.FileStorage{Path: options.DataDirectory}
	} else {
		storage = certmagic.Default.Storage
	}

	zapLogger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(boxtls.ACMEEncoderConfig()),
		&boxtls.ACMELogWriter{Logger: logger},
		zap.DebugLevel,
	))

	config := &certmagic.Config{
		DefaultServerName: options.DefaultServerName,
		Storage:           storage,
		Logger:            zapLogger,
		ReusePrivateKeys:  options.ReusePrivateKeys,
		MustStaple:        options.MustStaple,
	}
	if options.RenewalWindowRatio > 0 {
		config.RenewalWindowRatio = options.RenewalWindowRatio
	}
	if options.DisableOCSPStapling || len(options.OCSPOverrides) > 0 {
		config.OCSP = certmagic.OCSPConfig{
			DisableStapling:    options.DisableOCSPStapling,
			ResponderOverrides: maps.Clone(options.OCSPOverrides),
		}
	}
	if options.KeyType != "" {
		keyGenerator, err := createKeyGenerator(options.KeyType)
		if err != nil {
			return nil, err
		}
		config.KeySource = keyGenerator
	}

	acmeIssuer := certmagic.ACMEIssuer{
		CA:                      acmeServer,
		TestCA:                  options.TestCA,
		Email:                   options.Email,
		AccountKeyPEM:           options.AccountKey,
		Profile:                 options.Profile,
		NotAfter:                time.Duration(options.CertificateLifetime),
		Agreed:                  true,
		DisableHTTPChallenge:    options.DisableHTTPChallenge,
		DisableTLSALPNChallenge: options.DisableTLSALPNChallenge,
		AltHTTPPort:             int(options.AlternativeHTTPPort),
		AltTLSALPNPort:          int(options.AlternativeTLSPort),
		ListenHost:              options.BindHost,
		CertObtainTimeout:       time.Duration(options.ACMETimeout),
		Logger:                  zapLogger,
	}
	if len(options.TrustedRootsPEMFiles) > 0 {
		rootPool, err := loadTrustedRoots(options.TrustedRootsPEMFiles)
		if err != nil {
			return nil, err
		}
		acmeIssuer.TrustedRoots = rootPool
	}
	dnsSolver, err := newDNSSolver(options.DNS01Challenge, zapLogger)
	if err != nil {
		return nil, err
	}
	if dnsSolver != nil {
		acmeIssuer.DNS01Solver = dnsSolver
	}
	if options.ExternalAccount != nil && options.ExternalAccount.KeyID != "" {
		acmeIssuer.ExternalAccount = (*acme.EAB)(options.ExternalAccount)
	}
	if options.PreferredChains != nil {
		preferredChains, err := buildPreferredChains(options.PreferredChains)
		if err != nil {
			return nil, err
		}
		acmeIssuer.PreferredChains = preferredChains
	}
	if acmeServer == certmagic.ZeroSSLProductionCA {
		acmeIssuer.NewAccountFunc = func(ctx context.Context, acmeIssuer *certmagic.ACMEIssuer, account acme.Account) (acme.Account, error) {
			if acmeIssuer.ExternalAccount != nil {
				return account, nil
			}
			var err error
			acmeIssuer.ExternalAccount, account, err = createZeroSSLExternalAccountBinding(ctx, acmeIssuer, account)
			return account, err
		}
	}

	config.Issuers = []certmagic.Issuer{certmagic.NewACMEIssuer(config, acmeIssuer)}
	cache := certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(certificate certmagic.Certificate) (*certmagic.Config, error) {
			return config, nil
		},
		Logger: zapLogger,
	})
	config = certmagic.New(cache, *config)

	var nextProtos []string
	if !acmeIssuer.DisableTLSALPNChallenge && acmeIssuer.DNS01Solver == nil {
		nextProtos = []string{C.ACMETLS1Protocol}
	}
	return &Service{
		Adapter:    boxService.NewAdapter(C.TypeACME, tag),
		ctx:        ctx,
		config:     config,
		cache:      cache,
		domain:     options.Domain,
		nextProtos: nextProtos,
	}, nil
}

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return s.config.ManageAsync(s.ctx, s.domain)
}

func (s *Service) Close() error {
	if s.cache != nil {
		s.cache.Stop()
	}
	return nil
}

func (s *Service) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return s.config.GetCertificate(hello)
}

func (s *Service) GetACMENextProtos() []string {
	return s.nextProtos
}

func resolveACMEServer(provider string) (string, error) {
	switch provider {
	case "", "letsencrypt":
		return certmagic.LetsEncryptProductionCA, nil
	case "zerossl":
		return certmagic.ZeroSSLProductionCA, nil
	default:
		if !strings.HasPrefix(provider, "https://") {
			return "", E.New("unsupported ACME provider: ", provider)
		}
		return provider, nil
	}
}

func createKeyGenerator(keyType option.ACMEKeyType) (certmagic.StandardKeyGenerator, error) {
	switch keyType {
	case option.ACMEKeyTypeED25519:
		return certmagic.StandardKeyGenerator{KeyType: certmagic.ED25519}, nil
	case option.ACMEKeyTypeP256:
		return certmagic.StandardKeyGenerator{KeyType: certmagic.P256}, nil
	case option.ACMEKeyTypeP384:
		return certmagic.StandardKeyGenerator{KeyType: certmagic.P384}, nil
	case option.ACMEKeyTypeRSA2048:
		return certmagic.StandardKeyGenerator{KeyType: certmagic.RSA2048}, nil
	case option.ACMEKeyTypeRSA4096:
		return certmagic.StandardKeyGenerator{KeyType: certmagic.RSA4096}, nil
	default:
		return certmagic.StandardKeyGenerator{}, E.New("unsupported ACME key type: ", keyType)
	}
}

func newDNSSolver(dnsOptions *option.ACMEServiceDNS01ChallengeOptions, logger *zap.Logger) (*certmagic.DNS01Solver, error) {
	if dnsOptions == nil || dnsOptions.Provider == "" {
		return nil, nil
	}
	if dnsOptions.TTL < 0 {
		return nil, E.New("invalid ACME DNS01 ttl: ", dnsOptions.TTL)
	}
	if dnsOptions.PropagationDelay < 0 {
		return nil, E.New("invalid ACME DNS01 propagation_delay: ", dnsOptions.PropagationDelay)
	}
	if dnsOptions.PropagationTimeout < -1 {
		return nil, E.New("invalid ACME DNS01 propagation_timeout: ", dnsOptions.PropagationTimeout)
	}
	solver := &certmagic.DNS01Solver{
		DNSManager: certmagic.DNSManager{
			TTL:                time.Duration(dnsOptions.TTL),
			PropagationDelay:   time.Duration(dnsOptions.PropagationDelay),
			PropagationTimeout: time.Duration(dnsOptions.PropagationTimeout),
			Resolvers:          dnsOptions.Resolvers,
			OverrideDomain:     dnsOptions.OverrideDomain,
			Logger:             logger.Named("dns_manager"),
		},
	}
	switch dnsOptions.Provider {
	case C.DNSProviderAliDNS:
		solver.DNSProvider = &alidns.Provider{
			CredentialInfo: alidns.CredentialInfo{
				AccessKeyID:     dnsOptions.AliDNSOptions.AccessKeyID,
				AccessKeySecret: dnsOptions.AliDNSOptions.AccessKeySecret,
				RegionID:        dnsOptions.AliDNSOptions.RegionID,
				SecurityToken:   dnsOptions.AliDNSOptions.SecurityToken,
			},
		}
	case C.DNSProviderCloudflare:
		solver.DNSProvider = &cloudflare.Provider{
			APIToken:  dnsOptions.CloudflareOptions.APIToken,
			ZoneToken: dnsOptions.CloudflareOptions.ZoneToken,
		}
	case C.DNSProviderACMEDNS:
		solver.DNSProvider = &acmedns.Provider{
			Username:  dnsOptions.ACMEDNSOptions.Username,
			Password:  dnsOptions.ACMEDNSOptions.Password,
			Subdomain: dnsOptions.ACMEDNSOptions.Subdomain,
			ServerURL: dnsOptions.ACMEDNSOptions.ServerURL,
		}
	default:
		return nil, E.New("unsupported ACME DNS01 provider type: ", dnsOptions.Provider)
	}
	return solver, nil
}

func buildPreferredChains(options *option.ACMEPreferredChainsOptions) (certmagic.ChainPreference, error) {
	if len(options.RootCommonName) > 0 && len(options.AnyCommonName) > 0 {
		return certmagic.ChainPreference{}, E.New("preferred_chains.root_common_name and preferred_chains.any_common_name are mutually exclusive")
	}
	if !options.Smallest && len(options.RootCommonName) == 0 && len(options.AnyCommonName) == 0 {
		return certmagic.ChainPreference{}, E.New("preferred_chains is empty")
	}
	preferredChains := certmagic.ChainPreference{
		RootCommonName: options.RootCommonName,
		AnyCommonName:  options.AnyCommonName,
	}
	if options.Smallest {
		smallest := true
		preferredChains.Smallest = &smallest
	}
	return preferredChains, nil
}

func loadTrustedRoots(pemFiles []string) (*x509.CertPool, error) {
	rootPool := x509.NewCertPool()
	for _, pemFile := range pemFiles {
		pemContent, err := os.ReadFile(pemFile)
		if err != nil {
			return nil, E.Cause(err, "load trusted ACME root CA PEM file: ", pemFile)
		}
		if !rootPool.AppendCertsFromPEM(pemContent) {
			return nil, E.New("invalid trusted ACME root CA PEM file: ", pemFile)
		}
	}
	return rootPool, nil
}

func createZeroSSLExternalAccountBinding(ctx context.Context, acmeIssuer *certmagic.ACMEIssuer, account acme.Account) (*acme.EAB, acme.Account, error) {
	email := strings.TrimSpace(acmeIssuer.Email)
	if email == "" {
		return nil, acme.Account{}, E.New("email is required to use the ZeroSSL ACME endpoint without external_account")
	}
	if len(account.Contact) == 0 {
		account.Contact = []string{"mailto:" + email}
	}
	if acmeIssuer.CertObtainTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, acmeIssuer.CertObtainTimeout)
		defer cancel()
	}

	form := url.Values{"email": []string{email}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, zerossl.BaseURL+"/acme/eab-credentials-email", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, account, E.Cause(err, "create ZeroSSL EAB request")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", certmagic.UserAgent)

	response, err := newACMEHTTPClient(acmeIssuer).Do(request)
	if err != nil {
		return nil, account, E.Cause(err, "request ZeroSSL EAB")
	}
	defer response.Body.Close()

	var result struct {
		Success bool `json:"success"`
		Error   struct {
			Code int    `json:"code"`
			Type string `json:"type"`
		} `json:"error"`
		EABKID     string `json:"eab_kid"`
		EABHMACKey string `json:"eab_hmac_key"`
	}
	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		return nil, account, E.Cause(err, "decode ZeroSSL EAB response")
	}
	if response.StatusCode != http.StatusOK {
		return nil, account, E.New("failed getting ZeroSSL EAB credentials: HTTP ", response.StatusCode)
	}
	if result.Error.Code != 0 {
		return nil, account, E.New("failed getting ZeroSSL EAB credentials: ", result.Error.Type, " (code ", result.Error.Code, ")")
	}

	acmeIssuer.Logger.Info("generated ZeroSSL EAB credentials", zap.String("key_id", result.EABKID))

	return &acme.EAB{
		KeyID:  result.EABKID,
		MACKey: result.EABHMACKey,
	}, account, nil
}

func newACMEHTTPClient(acmeIssuer *certmagic.ACMEIssuer) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 2 * time.Minute,
	}
	if acmeIssuer.Resolver != "" {
		dialer.Resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{
					Timeout: 15 * time.Second,
				}).DialContext(ctx, network, acmeIssuer.Resolver)
			},
		}
	}
	transport := &http.Transport{
		Proxy:                 acmeIssuer.HTTPProxy,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 2 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	if acmeIssuer.TrustedRoots != nil {
		transport.TLSClientConfig = &tls.Config{
			RootCAs: acmeIssuer.TrustedRoots,
		}
	}
	httpTimeout := certmagic.HTTPTimeout
	if acmeIssuer.CertObtainTimeout > 0 {
		httpTimeout = acmeIssuer.CertObtainTimeout
	}
	return &http.Client{
		Transport: transport,
		Timeout:   httpTimeout,
	}
}
