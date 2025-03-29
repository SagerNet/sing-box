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

type acmeLogWriter struct {
	logger logger.Logger
}

func (w *acmeLogWriter) Write(p []byte) (n int, err error) {
	logLine := strings.ReplaceAll(string(p), "	", ": ")
	switch {
	case strings.HasPrefix(logLine, "error: "):
		w.logger.Error(logLine[7:])
	case strings.HasPrefix(logLine, "warn: "):
		w.logger.Warn(logLine[6:])
	case strings.HasPrefix(logLine, "info: "):
		w.logger.Info(logLine[6:])
	case strings.HasPrefix(logLine, "debug: "):
		w.logger.Debug(logLine[7:])
	default:
		w.logger.Debug(logLine)
	}
	return len(p), nil
}

func (w *acmeLogWriter) Sync() error {
	return nil
}

func encoderConfig() zapcore.EncoderConfig {
	config := zap.NewProductionEncoderConfig()
	config.TimeKey = zapcore.OmitKey
	return config
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
		zapcore.NewConsoleEncoder(encoderConfig()),
		&acmeLogWriter{logger: logger},
		zap.DebugLevel,
	))
	config := &certmagic.Config{
		DefaultServerName: options.DefaultServerName,
		Storage:           storage,
		Logger:            zapLogger,
	}
	acmeConfig := certmagic.ACMEIssuer{
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
			NextProtos:     []string{ACMETLS1Protocol},
		}
	}
	return tlsConfig, &acmeWrapper{ctx: ctx, cfg: config, cache: cache, domain: options.Domain}, nil
}
