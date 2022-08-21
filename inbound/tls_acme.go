//go:build with_acme

package inbound

import (
	"context"
	"crypto/tls"
	"strings"

	"github.com/sagernet/certmagic"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

type acmeWrapper struct {
	ctx    context.Context
	cfg    *certmagic.Config
	domain []string
}

func (w *acmeWrapper) Start() error {
	return w.cfg.ManageSync(w.ctx, w.domain)
}

func (w *acmeWrapper) Close() error {
	w.cfg.Unmanage(w.domain)
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
	}
	config.Issuers = []certmagic.Issuer{
		certmagic.NewACMEIssuer(config, certmagic.ACMEIssuer{
			CA:                      acmeServer,
			Email:                   options.Email,
			Agreed:                  true,
			DisableHTTPChallenge:    options.DisableHTTPChallenge,
			DisableTLSALPNChallenge: options.DisableTLSALPNChallenge,
			AltHTTPPort:             int(options.AlternativeHTTPPort),
			AltTLSALPNPort:          int(options.AlternativeTLSPort),
		}),
	}
	config = certmagic.New(certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(certificate certmagic.Certificate) (*certmagic.Config, error) {
			return config, nil
		},
	}), *config)
	return config.TLSConfig(), &acmeWrapper{ctx, config, options.Domain}, nil
}
