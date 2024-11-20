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
	"github.com/sagernet/fswatch"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/ntp"
)

type echServerConfig struct {
	config          *cftls.Config
	logger          log.Logger
	certificate     []byte
	key             []byte
	certificatePath string
	keyPath         string
	echKeyPath      string
	watcher         *fswatch.Watcher
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
	err := c.startWatcher()
	if err != nil {
		c.logger.Warn("create credentials watcher: ", err)
	}
	return nil
}

func (c *echServerConfig) startWatcher() error {
	var watchPath []string
	if c.certificatePath != "" {
		watchPath = append(watchPath, c.certificatePath)
	}
	if c.keyPath != "" {
		watchPath = append(watchPath, c.keyPath)
	}
	if c.echKeyPath != "" {
		watchPath = append(watchPath, c.echKeyPath)
	}
	if len(watchPath) == 0 {
		return nil
	}
	watcher, err := fswatch.NewWatcher(fswatch.Options{
		Path: watchPath,
		Callback: func(path string) {
			err := c.credentialsUpdated(path)
			if err != nil {
				c.logger.Error(E.Cause(err, "reload credentials from ", path))
			}
		},
	})
	if err != nil {
		return err
	}
	err = watcher.Start()
	if err != nil {
		return err
	}
	c.watcher = watcher
	return nil
}

func (c *echServerConfig) credentialsUpdated(path string) error {
	if path == c.certificatePath || path == c.keyPath {
		if path == c.certificatePath {
			certificate, err := os.ReadFile(c.certificatePath)
			if err != nil {
				return err
			}
			c.certificate = certificate
		} else {
			key, err := os.ReadFile(c.keyPath)
			if err != nil {
				return err
			}
			c.key = key
		}
		keyPair, err := cftls.X509KeyPair(c.certificate, c.key)
		if err != nil {
			return E.Cause(err, "parse key pair")
		}
		c.config.Certificates = []cftls.Certificate{keyPair}
		c.logger.Info("reloaded TLS certificate")
	} else {
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
	}
	return nil
}

func (c *echServerConfig) Close() error {
	var err error
	if c.watcher != nil {
		err = E.Append(err, c.watcher.Close(), func(err error) error {
			return E.Cause(err, "close credentials watcher")
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
	} else if options.ECH.KeyPath != "" {
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
