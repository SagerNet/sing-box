//go:build with_acme

package tls

import (
	"context"
	"crypto/tls"
	"os"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/alidns"
	"github.com/libdns/cloudflare"
	"github.com/mholt/acmez/acme"
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

func startACME(ctx context.Context, options option.InboundACMEOptions) (*tls.Config, adapter.Service, error) {
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
	config := &certmagic.Config{
		DefaultServerName: options.DefaultServerName,
		Storage:           storage,
		Logger: zap.New(zapcore.NewCore(
			zapcore.NewConsoleEncoder(zap.NewProductionEncoderConfig()),
			os.Stderr,
			zap.InfoLevel,
		)),
	}
	acmeConfig := certmagic.ACMEIssuer{
		CA:                      acmeServer,
		Email:                   options.Email,
		Agreed:                  true,
		DisableHTTPChallenge:    options.DisableHTTPChallenge,
		DisableTLSALPNChallenge: options.DisableTLSALPNChallenge,
		AltHTTPPort:             int(options.AlternativeHTTPPort),
		AltTLSALPNPort:          int(options.AlternativeTLSPort),
		Logger:                  config.Logger,
	}
	if dnsOptions := options.DNS01Challenge; dnsOptions != nil && dnsOptions.Provider != "" {
		var solver certmagic.DNS01Solver
		switch dnsOptions.Provider {
		case C.DNSProviderAliDNS:
			solver.DNSProvider = &alidns.Provider{
				AccKeyID:     dnsOptions.AliDNSOptions.AccessKeyID,
				AccKeySecret: dnsOptions.AliDNSOptions.AccessKeySecret,
				RegionID:     dnsOptions.AliDNSOptions.RegionID,
			}
		case C.DNSProviderCloudflare:
			solver.DNSProvider = &cloudflare.Provider{
				APIToken: dnsOptions.CloudflareOptions.APIToken,
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
	})
	config = certmagic.New(cache, *config)
	return config.TLSConfig(), &acmeWrapper{ctx: ctx, cfg: config, cache: cache, domain: options.Domain}, nil
}
