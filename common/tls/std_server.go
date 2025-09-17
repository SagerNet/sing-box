package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/fswatch"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/ntp"
)

var errInsecureUnused = E.New("tls: insecure unused")

type STDServerConfig struct {
	access                sync.RWMutex
	config                *tls.Config
	logger                log.Logger
	acmeService           adapter.SimpleLifecycle
	certificate           []byte
	key                   []byte
	certificatePath       string
	keyPath               string
	clientCertificatePath []string
	echKeyPath            string
	watcher               *fswatch.Watcher
}

func (c *STDServerConfig) ServerName() string {
	c.access.RLock()
	defer c.access.RUnlock()
	return c.config.ServerName
}

func (c *STDServerConfig) SetServerName(serverName string) {
	c.access.Lock()
	defer c.access.Unlock()
	config := c.config.Clone()
	config.ServerName = serverName
	c.config = config
}

func (c *STDServerConfig) NextProtos() []string {
	c.access.RLock()
	defer c.access.RUnlock()
	if c.acmeService != nil && len(c.config.NextProtos) > 1 && c.config.NextProtos[0] == ACMETLS1Protocol {
		return c.config.NextProtos[1:]
	} else {
		return c.config.NextProtos
	}
}

func (c *STDServerConfig) SetNextProtos(nextProto []string) {
	c.access.Lock()
	defer c.access.Unlock()
	config := c.config.Clone()
	if c.acmeService != nil && len(c.config.NextProtos) > 1 && c.config.NextProtos[0] == ACMETLS1Protocol {
		config.NextProtos = append(c.config.NextProtos[:1], nextProto...)
	} else {
		config.NextProtos = nextProto
	}
	c.config = config
}

func (c *STDServerConfig) STDConfig() (*STDConfig, error) {
	return c.config, nil
}

func (c *STDServerConfig) Client(conn net.Conn) (Conn, error) {
	return tls.Client(conn, c.config), nil
}

func (c *STDServerConfig) Server(conn net.Conn) (Conn, error) {
	return tls.Server(conn, c.config), nil
}

func (c *STDServerConfig) Clone() Config {
	return &STDServerConfig{
		config: c.config.Clone(),
	}
}

func (c *STDServerConfig) Start() error {
	if c.acmeService != nil {
		return c.acmeService.Start()
	} else {
		err := c.startWatcher()
		if err != nil {
			c.logger.Warn("create fsnotify watcher: ", err)
		}
		return nil
	}
}

func (c *STDServerConfig) startWatcher() error {
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
	if len(c.clientCertificatePath) > 0 {
		watchPath = append(watchPath, c.clientCertificatePath...)
	}
	if len(watchPath) == 0 {
		return nil
	}
	watcher, err := fswatch.NewWatcher(fswatch.Options{
		Path: watchPath,
		Callback: func(path string) {
			err := c.certificateUpdated(path)
			if err != nil {
				c.logger.Error(E.Cause(err, "reload certificate"))
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

func (c *STDServerConfig) certificateUpdated(path string) error {
	if path == c.certificatePath || path == c.keyPath {
		if path == c.certificatePath {
			certificate, err := os.ReadFile(c.certificatePath)
			if err != nil {
				return E.Cause(err, "reload certificate from ", c.certificatePath)
			}
			c.certificate = certificate
		} else if path == c.keyPath {
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
		c.access.Lock()
		config := c.config.Clone()
		config.Certificates = []tls.Certificate{keyPair}
		c.config = config
		c.access.Unlock()
		c.logger.Info("reloaded TLS certificate")
	} else if common.Contains(c.clientCertificatePath, path) {
		clientCertificateCA := x509.NewCertPool()
		var reloaded bool
		for _, certPath := range c.clientCertificatePath {
			content, err := os.ReadFile(certPath)
			if err != nil {
				c.logger.Error(E.Cause(err, "reload certificate from ", c.clientCertificatePath))
				continue
			}
			if !clientCertificateCA.AppendCertsFromPEM(content) {
				c.logger.Error(E.New("invalid client certificate file: ", certPath))
				continue
			}
			reloaded = true
		}
		if !reloaded {
			return E.New("client certificates is empty")
		}
		c.access.Lock()
		config := c.config.Clone()
		config.ClientCAs = clientCertificateCA
		c.config = config
		c.access.Unlock()
		c.logger.Info("reloaded client certificates")
	} else if path == c.echKeyPath {
		echKey, err := os.ReadFile(c.echKeyPath)
		if err != nil {
			return E.Cause(err, "reload ECH keys from ", c.echKeyPath)
		}
		err = c.setECHServerConfig(echKey)
		if err != nil {
			return err
		}
		c.logger.Info("reloaded ECH keys")
	}
	return nil
}

func (c *STDServerConfig) Close() error {
	if c.acmeService != nil {
		return c.acmeService.Close()
	}
	if c.watcher != nil {
		return c.watcher.Close()
	}
	return nil
}

func NewSTDServer(ctx context.Context, logger log.ContextLogger, options option.InboundTLSOptions) (ServerConfig, error) {
	if !options.Enabled {
		return nil, nil
	}
	var tlsConfig *tls.Config
	var acmeService adapter.SimpleLifecycle
	var err error
	if options.ACME != nil && len(options.ACME.Domain) > 0 {
		//nolint:staticcheck
		tlsConfig, acmeService, err = startACME(ctx, logger, common.PtrValueOrDefault(options.ACME))
		if err != nil {
			return nil, err
		}
		if options.Insecure {
			return nil, errInsecureUnused
		}
	} else {
		tlsConfig = &tls.Config{}
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
	for _, curveID := range options.CurvePreferences {
		tlsConfig.CurvePreferences = append(tlsConfig.CurvePreferences, tls.CurveID(curveID))
	}
	tlsConfig.ClientAuth = tls.ClientAuthType(options.ClientAuthentication)
	var (
		certificate []byte
		key         []byte
	)
	if acmeService == nil {
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
		if certificate == nil && key == nil && options.Insecure {
			timeFunc := ntp.TimeFuncFromContext(ctx)
			if timeFunc == nil {
				timeFunc = time.Now
			}
			tlsConfig.GetCertificate = func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				return GenerateKeyPair(nil, nil, timeFunc, info.ServerName)
			}
		} else {
			if certificate == nil {
				return nil, E.New("missing certificate")
			} else if key == nil {
				return nil, E.New("missing key")
			}

			keyPair, err := tls.X509KeyPair(certificate, key)
			if err != nil {
				return nil, E.Cause(err, "parse x509 key pair")
			}
			tlsConfig.Certificates = []tls.Certificate{keyPair}
		}
	}
	if len(options.ClientCertificate) > 0 || len(options.ClientCertificatePath) > 0 {
		if tlsConfig.ClientAuth == tls.NoClientCert {
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
	}
	if tlsConfig.ClientAuth == tls.VerifyClientCertIfGiven || tlsConfig.ClientAuth == tls.RequireAndVerifyClientCert {
		if len(options.ClientCertificate) > 0 {
			clientCertificateCA := x509.NewCertPool()
			if !clientCertificateCA.AppendCertsFromPEM([]byte(strings.Join(options.ClientCertificate, "\n"))) {
				return nil, E.New("invalid client certificate strings")
			}
			tlsConfig.ClientCAs = clientCertificateCA
		} else if len(options.ClientCertificatePath) > 0 {
			clientCertificateCA := x509.NewCertPool()
			for _, path := range options.ClientCertificatePath {
				content, err := os.ReadFile(path)
				if err != nil {
					return nil, E.Cause(err, "read client certificate from ", path)
				}
				if !clientCertificateCA.AppendCertsFromPEM(content) {
					return nil, E.New("invalid client certificate file: ", path)
				}
			}
			tlsConfig.ClientCAs = clientCertificateCA
		} else if len(options.ClientCertificatePublicKeySHA256) > 0 {
			if tlsConfig.ClientAuth == tls.RequireAndVerifyClientCert {
				tlsConfig.ClientAuth = tls.RequireAnyClientCert
			} else if tlsConfig.ClientAuth == tls.VerifyClientCertIfGiven {
				tlsConfig.ClientAuth = tls.RequestClientCert
			}
			tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				return verifyPublicKeySHA256(options.ClientCertificatePublicKeySHA256, rawCerts, tlsConfig.Time)
			}
		} else {
			return nil, E.New("missing client_certificate, client_certificate_path or client_certificate_public_key_sha256 for client authentication")
		}
	}
	var echKeyPath string
	if options.ECH != nil && options.ECH.Enabled {
		err = parseECHServerConfig(ctx, options, tlsConfig, &echKeyPath)
		if err != nil {
			return nil, err
		}
	}
	serverConfig := &STDServerConfig{
		config:                tlsConfig,
		logger:                logger,
		acmeService:           acmeService,
		certificate:           certificate,
		key:                   key,
		certificatePath:       options.CertificatePath,
		clientCertificatePath: options.ClientCertificatePath,
		keyPath:               options.KeyPath,
		echKeyPath:            echKeyPath,
	}
	serverConfig.config.GetConfigForClient = func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		serverConfig.access.Lock()
		defer serverConfig.access.Unlock()
		return serverConfig.config, nil
	}
	var config ServerConfig = serverConfig
	if options.KernelTx || options.KernelRx {
		if !C.IsLinux {
			return nil, E.New("kTLS is only supported on Linux")
		}
		config = &KTlSServerConfig{
			ServerConfig: config,
			logger:       logger,
			kernelTx:     options.KernelTx,
			kernelRx:     options.KernelRx,
		}
	}
	return config, nil
}
