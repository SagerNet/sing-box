//go:build with_acme

package tls

import (
	"context"
	"crypto/tls"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/acmedns"
	"github.com/libdns/alidns"
	"github.com/libdns/cloudflare"
	"github.com/mholt/acmez/v3/acme"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type acmeWrapper struct {
	ctx    context.Context
	cfg    *certmagic.Config
	cache  *certmagic.Cache
	domain []string
}

func (w *acmeWrapper) Start() error {
	return w.cfg.ManageSync(w.ctx, w.domain)
}

func (w *acmeWrapper) Close() error {
	w.cache.Stop()
	return nil
}

func startACME(ctx context.Context, logger logger.Logger, options option.InboundACMEOptions) (*tls.Config, adapter.SimpleLifecycle, error) {
	var acmeServer string
	switch options.Provider {
	case "", "letsencrypt":
		acmeServer = certmagic.LetsEncryptProductionCA
	case "zerossl":
		acmeServer = certmagic.ZeroSSLProductionCA
	default:
		if !strings.HasPrefix(options.Provider, "https://") {
			return nil, nil, E.New("unsupported acme provider: " + options.Provider)
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
		zapcore.NewConsoleEncoder(ACMEEncoderConfig()),
		&ACMELogWriter{Logger: logger},
		zap.DebugLevel,
	))
	config := &certmagic.Config{
		DefaultServerName: options.DefaultServerName,
		Storage:           storage,
		Logger:            zapLogger,
	}
	profile := options.Profile
	if profile == "" && acmeServer == certmagic.LetsEncryptProductionCA {
		for _, domain := range options.Domain {
			if certmagic.SubjectIsIP(domain) {
				profile = "shortlived"
				break
			}
		}
	}

	acmeConfig := certmagic.ACMEIssuer{
		CA:                      acmeServer,
		Email:                   options.Email,
		Agreed:                  true,
		Profile:                 profile,
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
			return nil, nil, E.New("unsupported ACME DNS01 provider type: " + dnsOptions.Provider)
		}
		acmeConfig.DNS01Solver = &solver
	}
	if options.ExternalAccount != nil && options.ExternalAccount.KeyID != "" {
		acmeConfig.ExternalAccount = (*acme.EAB)(options.ExternalAccount)
	}
	config.Issuers = []certmagic.Issuer{certmagic.NewACMEIssuer(config, acmeConfig)}
	cache := certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(certificate certmagic.Certificate) (*certmagic.Config, error) {
			return config, nil
		},
		Logger: zapLogger,
	})
	config = certmagic.New(cache, *config)
	var tlsConfig *tls.Config
	if acmeConfig.DisableTLSALPNChallenge || acmeConfig.DNS01Solver != nil {
		tlsConfig = &tls.Config{
			GetCertificate: config.GetCertificate,
		}
	} else {
		tlsConfig = &tls.Config{
			GetCertificate: config.GetCertificate,
			NextProtos:     []string{C.ACMETLS1Protocol},
		}
	}
	return tlsConfig, &acmeWrapper{ctx: ctx, cfg: config, cache: cache, domain: options.Domain}, nil
}
