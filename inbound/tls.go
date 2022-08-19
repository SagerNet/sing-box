package inbound

import (
	"context"
	"crypto/tls"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/fsnotify/fsnotify"
)

var _ adapter.Service = (*TLSConfig)(nil)

type TLSConfig struct {
	config          *tls.Config
	logger          log.Logger
	acmeService     adapter.Service
	certificate     []byte
	key             []byte
	certificatePath string
	keyPath         string
	watcher         *fsnotify.Watcher
}

func (c *TLSConfig) Config() *tls.Config {
	return c.config
}

func (c *TLSConfig) Start() error {
	if c.acmeService != nil {
		return c.acmeService.Start()
	} else {
		if c.certificatePath == "" && c.keyPath == "" {
			return nil
		}
		err := c.startWatcher()
		if err != nil {
			c.logger.Warn("create fsnotify watcher: ", err)
		}
		return nil
	}
}

func (c *TLSConfig) startWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if c.certificatePath != "" {
		err = watcher.Add(c.certificatePath)
		if err != nil {
			return err
		}
	}
	if c.keyPath != "" {
		err = watcher.Add(c.keyPath)
		if err != nil {
			return err
		}
	}
	c.watcher = watcher
	go c.loopUpdate()
	return nil
}

func (c *TLSConfig) loopUpdate() {
	for {
		select {
		case event, ok := <-c.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write != fsnotify.Write {
				continue
			}
			err := c.reloadKeyPair()
			if err != nil {
				c.logger.Error(E.Cause(err, "reload TLS key pair"))
			}
		case err, ok := <-c.watcher.Errors:
			if !ok {
				return
			}
			c.logger.Error(E.Cause(err, "fsnotify error"))
		}
	}
}

func (c *TLSConfig) reloadKeyPair() error {
	if c.certificatePath != "" {
		certificate, err := os.ReadFile(c.certificatePath)
		if err != nil {
			return E.Cause(err, "reload certificate from ", c.certificatePath)
		}
		c.certificate = certificate
	}
	if c.keyPath != "" {
		key, err := os.ReadFile(c.keyPath)
		if err != nil {
			return E.Cause(err, "reload key from ", c.keyPath)
		}
		c.key = key
	}
	keyPair, err := tls.X509KeyPair(c.certificate, c.key)
	if err != nil {
		return E.Cause(err, "reload key pair")
	}
	c.config.Certificates = []tls.Certificate{keyPair}
	c.logger.Info("reloaded TLS certificate")
	return nil
}

func (c *TLSConfig) Close() error {
	if c.acmeService != nil {
		return c.acmeService.Close()
	}
	if c.watcher != nil {
		return c.watcher.Close()
	}
	return nil
}

func NewTLSConfig(ctx context.Context, logger log.Logger, options option.InboundTLSOptions) (*TLSConfig, error) {
	if !options.Enabled {
		return nil, nil
	}
	var tlsConfig *tls.Config
	var acmeService adapter.Service
	var err error
	if options.ACME != nil && len(options.ACME.Domain) > 0 {
		tlsConfig, acmeService, err = startACME(ctx, common.PtrValueOrDefault(options.ACME))
		if err != nil {
			return nil, err
		}
	} else {
		tlsConfig = &tls.Config{}
	}
	tlsConfig.NextProtos = []string{}
	if options.ServerName != "" {
		tlsConfig.ServerName = options.ServerName
	}
	if len(options.ALPN) > 0 {
		tlsConfig.NextProtos = options.ALPN
	}
	if options.MinVersion != "" {
		minVersion, err := option.ParseTLSVersion(options.MinVersion)
		if err != nil {
			return nil, E.Cause(err, "parse min_version")
		}
		tlsConfig.MinVersion = minVersion
	}
	if options.MaxVersion != "" {
		maxVersion, err := option.ParseTLSVersion(options.MaxVersion)
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
	var key []byte
	if acmeService == nil {
		if options.Certificate != "" {
			certificate = []byte(options.Certificate)
		} else if options.CertificatePath != "" {
			content, err := os.ReadFile(options.CertificatePath)
			if err != nil {
				return nil, E.Cause(err, "read certificate")
			}
			certificate = content
		}
		if options.Key != "" {
			key = []byte(options.Key)
		} else if options.KeyPath != "" {
			content, err := os.ReadFile(options.KeyPath)
			if err != nil {
				return nil, E.Cause(err, "read key")
			}
			key = content
		}
		if certificate == nil {
			return nil, E.New("missing certificate")
		}
		if key == nil {
			return nil, E.New("missing key")
		}
		keyPair, err := tls.X509KeyPair(certificate, key)
		if err != nil {
			return nil, E.Cause(err, "parse x509 key pair")
		}
		tlsConfig.Certificates = []tls.Certificate{keyPair}
	}
	return &TLSConfig{
		config:          tlsConfig,
		logger:          logger,
		acmeService:     acmeService,
		certificate:     certificate,
		key:             key,
		certificatePath: options.CertificatePath,
		keyPath:         options.KeyPath,
	}, nil
}
