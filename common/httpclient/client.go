package httpclient

import (
	"context"
	"time"

	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
)

func NewTransport(ctx context.Context, logger logger.ContextLogger, tag string, options option.HTTPClientOptions) (*ManagedTransport, error) {
	rawDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:                 ctx,
		Options:                 options.DialerOptions,
		RemoteIsDomain:          true,
		DirectResolver:          options.DirectResolver,
		ResolverOnDetour:        options.ResolveOnDetour,
		NewDialer:               options.ResolveOnDetour,
		DisableEmptyDirectCheck: options.DisableEmptyDirectCheck,
		DefaultOutbound:         options.DefaultOutbound,
	})
	if err != nil {
		return nil, err
	}
	headers := options.Headers.Build()
	host := headers.Get("Host")
	headers.Del("Host")

	var cheapRebuild bool
	switch options.Engine {
	case C.TLSEngineApple:
		inner, transportErr := newAppleTransport(ctx, logger, rawDialer, options)
		if transportErr != nil {
			return nil, transportErr
		}
		managedTransport := &ManagedTransport{
			dialer:  rawDialer,
			headers: headers,
			host:    host,
			tag:     tag,
			factory: func() (innerTransport, error) {
				return newAppleTransport(ctx, logger, rawDialer, options)
			},
		}
		managedTransport.epoch.Store(&transportEpoch{transport: inner})
		return managedTransport, nil
	case "", C.TLSEngineGo:
		cheapRebuild = true
	default:
		return nil, E.New("unknown HTTP engine: ", options.Engine)
	}
	tlsOptions := common.PtrValueOrDefault(options.TLS)
	tlsOptions.Enabled = true
	baseTLSConfig, err := tls.NewClientWithOptions(tls.ClientOptions{
		Context:              ctx,
		Logger:               logger,
		Options:              tlsOptions,
		AllowEmptyServerName: true,
	})
	if err != nil {
		return nil, err
	}
	inner, err := newTransport(rawDialer, baseTLSConfig, options)
	if err != nil {
		return nil, err
	}
	managedTransport := &ManagedTransport{
		cheapRebuild: cheapRebuild,
		dialer:       rawDialer,
		headers:      headers,
		host:         host,
		tag:          tag,
		factory: func() (innerTransport, error) {
			return newTransport(rawDialer, baseTLSConfig, options)
		},
	}
	managedTransport.epoch.Store(&transportEpoch{transport: inner})
	return managedTransport, nil
}

func newTransport(rawDialer N.Dialer, baseTLSConfig tls.Config, options option.HTTPClientOptions) (innerTransport, error) {
	version := options.Version
	if version == 0 {
		version = 2
	}
	fallbackDelay := time.Duration(options.DialerOptions.FallbackDelay)
	if fallbackDelay == 0 {
		fallbackDelay = 300 * time.Millisecond
	}
	var transport innerTransport
	var err error
	switch version {
	case 1:
		transport = newHTTP1Transport(rawDialer, baseTLSConfig)
	case 2:
		if options.DisableVersionFallback {
			transport, err = newHTTP2Transport(rawDialer, baseTLSConfig, options.HTTP2Options)
		} else {
			transport, err = newHTTP2FallbackTransport(rawDialer, baseTLSConfig, options.HTTP2Options)
		}
	case 3:
		if baseTLSConfig != nil {
			_, err = baseTLSConfig.STDConfig()
			if err != nil {
				return nil, err
			}
		}
		if options.DisableVersionFallback {
			transport, err = newHTTP3Transport(rawDialer, baseTLSConfig, options.HTTP3Options)
		} else {
			var h2Fallback innerTransport
			h2Fallback, err = newHTTP2FallbackTransport(rawDialer, baseTLSConfig, options.HTTP2Options)
			if err != nil {
				return nil, err
			}
			transport, err = newHTTP3FallbackTransport(rawDialer, baseTLSConfig, h2Fallback, options.HTTP3Options, fallbackDelay)
		}
	default:
		return nil, E.New("unknown HTTP version: ", version)
	}
	if err != nil {
		return nil, err
	}
	return transport, nil
}
