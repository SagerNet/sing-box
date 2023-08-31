//go:build with_ech

package tls

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"net"
	"os"
	"strings"

	cftls "github.com/sagernet/cloudflare-tls"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/ntp"

	"github.com/fsnotify/fsnotify"
)

type echServerConfig struct {
	config          *cftls.Config
	logger          log.Logger
	certificate     []byte
	key             []byte
	certificatePath string
	keyPath         string
	watcher         *fsnotify.Watcher
	echKeyPath      string
	echWatcher      *fsnotify.Watcher
}

func (c *echServerConfig) ServerName() string {
	return c.config.ServerName
}

func (c *echServerConfig) SetServerName(serverName string) {
	c.config.ServerName = serverName
}

func (c *echServerConfig) NextProtos() []string {
	return c.config.NextProtos
}

func (c *echServerConfig) SetNextProtos(nextProto []string) {
	c.config.NextProtos = nextProto
}

func (c *echServerConfig) Config() (*STDConfig, error) {
	return nil, E.New("unsupported usage for ECH")
}

func (c *echServerConfig) Client(conn net.Conn) (Conn, error) {
	return &echConnWrapper{cftls.Client(conn, c.config)}, nil
}

func (c *echServerConfig) Server(conn net.Conn) (Conn, error) {
	return &echConnWrapper{cftls.Server(conn, c.config)}, nil
}

func (c *echServerConfig) Clone() Config {
	return &echServerConfig{
		config: c.config.Clone(),
	}
}

func (c *echServerConfig) Start() error {
	if c.certificatePath != "" && c.keyPath != "" {
		err := c.startWatcher()
		if err != nil {
			c.logger.Warn("create fsnotify watcher: ", err)
		}
	}
	if c.echKeyPath != "" {
		err := c.startECHWatcher()
		if err != nil {
			c.logger.Warn("create fsnotify watcher: ", err)
		}
	}
	return nil
}

func (c *echServerConfig) startWatcher() error {
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

func (c *echServerConfig) loopUpdate() {
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

func (c *echServerConfig) reloadKeyPair() error {
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
	keyPair, err := cftls.X509KeyPair(c.certificate, c.key)
	if err != nil {
		return E.Cause(err, "reload key pair")
	}
	c.config.Certificates = []cftls.Certificate{keyPair}
	c.logger.Info("reloaded TLS certificate")
	return nil
}

func (c *echServerConfig) startECHWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	err = watcher.Add(c.echKeyPath)
	if err != nil {
		return err
	}
	c.echWatcher = watcher
	go c.loopECHUpdate()
	return nil
}

func (c *echServerConfig) loopECHUpdate() {
	for {
		select {
		case event, ok := <-c.echWatcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write != fsnotify.Write {
				continue
			}
			err := c.reloadECHKey()
			if err != nil {
				c.logger.Error(E.Cause(err, "reload ECH key"))
			}
		case err, ok := <-c.echWatcher.Errors:
			if !ok {
				return
			}
			c.logger.Error(E.Cause(err, "fsnotify error"))
		}
	}
}

func (c *echServerConfig) reloadECHKey() error {
	echKeyContent, err := os.ReadFile(c.echKeyPath)
	if err != nil {
		return err
	}
	block, rest := pem.Decode(echKeyContent)
	if block == nil || block.Type != "ECH KEYS" || len(rest) > 0 {
		return E.New("invalid ECH keys pem")
	}
	echKeys, err := cftls.EXP_UnmarshalECHKeys(block.Bytes)
	if err != nil {
		return E.Cause(err, "parse ECH keys")
	}
	echKeySet, err := cftls.EXP_NewECHKeySet(echKeys)
	if err != nil {
		return E.Cause(err, "create ECH key set")
	}
	c.config.ServerECHProvider = echKeySet
	c.logger.Info("reloaded ECH keys")
	return nil
}

func (c *echServerConfig) Close() error {
	var err error
	if c.watcher != nil {
		err = E.Append(err, c.watcher.Close(), func(err error) error {
			return E.Cause(err, "close certificate watcher")
		})
	}
	if c.echWatcher != nil {
		err = E.Append(err, c.echWatcher.Close(), func(err error) error {
			return E.Cause(err, "close ECH key watcher")
		})
	}
	return err
}

func NewECHServer(ctx context.Context, logger log.Logger, options option.InboundTLSOptions) (ServerConfig, error) {
	if !options.Enabled {
		return nil, nil
	}
	var tlsConfig cftls.Config
	if options.ACME != nil && len(options.ACME.Domain) > 0 {
		return nil, E.New("acme is unavailable in ech")
	}
	tlsConfig.Time = ntp.TimeFuncFromContext(ctx)
	if options.ServerName != "" {
		tlsConfig.ServerName = options.ServerName
	}
	if len(options.ALPN) > 0 {
		tlsConfig.NextProtos = append(options.ALPN, tlsConfig.NextProtos...)
	}
	if options.MinVersion != "" {
		minVersion, err := ParseTLSVersion(options.MinVersion)
		if err != nil {
			return nil, E.Cause(err, "parse min_version")
		}
		tlsConfig.MinVersion = minVersion
	}
	if options.MaxVersion != "" {
		maxVersion, err := ParseTLSVersion(options.MaxVersion)
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
	if len(options.Certificate) > 0 {
		certificate = []byte(strings.Join(options.Certificate, "\n"))
	} else if options.CertificatePath != "" {
		content, err := os.ReadFile(options.CertificatePath)
		if err != nil {
			return nil, E.Cause(err, "read certificate")
		}
		certificate = content
	}
	if len(options.Key) > 0 {
		key = []byte(strings.Join(options.Key, "\n"))
	} else if options.KeyPath != "" {
		content, err := os.ReadFile(options.KeyPath)
		if err != nil {
			return nil, E.Cause(err, "read key")
		}
		key = content
	}

	if certificate == nil {
		return nil, E.New("missing certificate")
	} else if key == nil {
		return nil, E.New("missing key")
	}

	keyPair, err := cftls.X509KeyPair(certificate, key)
	if err != nil {
		return nil, E.Cause(err, "parse x509 key pair")
	}
	tlsConfig.Certificates = []cftls.Certificate{keyPair}

	var echKey []byte
	if len(options.ECH.Key) > 0 {
		echKey = []byte(strings.Join(options.ECH.Key, "\n"))
	} else if options.KeyPath != "" {
		content, err := os.ReadFile(options.ECH.KeyPath)
		if err != nil {
			return nil, E.Cause(err, "read ECH key")
		}
		echKey = content
	} else {
		return nil, E.New("missing ECH key")
	}

	block, rest := pem.Decode(echKey)
	if block == nil || block.Type != "ECH KEYS" || len(rest) > 0 {
		return nil, E.New("invalid ECH keys pem")
	}

	echKeys, err := cftls.EXP_UnmarshalECHKeys(block.Bytes)
	if err != nil {
		return nil, E.Cause(err, "parse ECH keys")
	}

	echKeySet, err := cftls.EXP_NewECHKeySet(echKeys)
	if err != nil {
		return nil, E.Cause(err, "create ECH key set")
	}

	tlsConfig.ECHEnabled = true
	tlsConfig.PQSignatureSchemesEnabled = options.ECH.PQSignatureSchemesEnabled
	tlsConfig.DynamicRecordSizingDisabled = options.ECH.DynamicRecordSizingDisabled
	tlsConfig.ServerECHProvider = echKeySet

	return &echServerConfig{
		config:          &tlsConfig,
		logger:          logger,
		certificate:     certificate,
		key:             key,
		certificatePath: options.CertificatePath,
		keyPath:         options.KeyPath,
		echKeyPath:      options.ECH.KeyPath,
	}, nil
}
