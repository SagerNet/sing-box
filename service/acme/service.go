//go:build with_acme

package acme

import (
	"context"
	"crypto/tls"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	boxtls "github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/caddyserver/certmagic"
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
	logger     log.ContextLogger
	config     *certmagic.Config
	cache      *certmagic.Cache
	domain     []string
	nextProtos []string
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.ACMEServiceOptions) (adapter.Service, error) {
	var acmeServer string
	switch options.Provider {
	case "", "letsencrypt":
		acmeServer = certmagic.LetsEncryptProductionCA
	case "zerossl":
		acmeServer = certmagic.ZeroSSLProductionCA
	default:
		if !strings.HasPrefix(options.Provider, "https://") {
			return nil, E.New("unsupported ACME provider: ", options.Provider)
		}
		acmeServer = options.Provider
	}
	var storage certmagic.Storage
	if options.DataDirectory != "" {
		storage = &certmagic.FileStorage{
			Path: options.DataDirectory,
		}
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
	}
	acmeIssuer := certmagic.ACMEIssuer{
		CA:                      acmeServer,
		Email:                   options.Email,
		Agreed:                  true,
		DisableHTTPChallenge:    options.DisableHTTPChallenge,
		DisableTLSALPNChallenge: options.DisableTLSALPNChallenge,
		AltHTTPPort:             int(options.AlternativeHTTPPort),
		AltTLSALPNPort:          int(options.AlternativeTLSPort),
		Logger:                  zapLogger,
	}
	if dnsOptions := options.DNS01Challenge; dnsOptions != nil && dnsOptions.Provider != "" {
		var solver certmagic.DNS01Solver
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
		acmeIssuer.DNS01Solver = &solver
	}
	if options.ExternalAccount != nil && options.ExternalAccount.KeyID != "" {
		acmeIssuer.ExternalAccount = (*acme.EAB)(options.ExternalAccount)
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
		logger:     logger,
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
