//go:build (darwin && cgo) || windows

package tls

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/ntp"
)

type systemTLSConfig struct {
	serverName                 string
	nextProtos                 []string
	handshakeTimeout           time.Duration
	minVersion                 uint16
	maxVersion                 uint16
	insecure                   bool
	anchorOnly                 bool
	certificatePublicKeySHA256 [][]byte
	timeFunc                   func() time.Time
	store                      adapter.CertificateStore
}

func (c *systemTLSConfig) ServerName() string {
	return c.serverName
}

func (c *systemTLSConfig) SetServerName(serverName string) {
	c.serverName = serverName
}

func (c *systemTLSConfig) NextProtos() []string {
	return c.nextProtos
}

func (c *systemTLSConfig) SetNextProtos(nextProto []string) {
	c.nextProtos = append([]string(nil), nextProto...)
}

func (c *systemTLSConfig) HandshakeTimeout() time.Duration {
	return c.handshakeTimeout
}

func (c *systemTLSConfig) SetHandshakeTimeout(timeout time.Duration) {
	c.handshakeTimeout = timeout
}

func (c *systemTLSConfig) STDConfig() (*STDConfig, error) {
	return nil, E.New("STDConfig is unsupported for the system TLS engine")
}

func (c *systemTLSConfig) Client(conn net.Conn) (Conn, error) {
	return nil, os.ErrInvalid
}

func (c *systemTLSConfig) clone() systemTLSConfig {
	return systemTLSConfig{
		serverName:                 c.serverName,
		nextProtos:                 append([]string(nil), c.nextProtos...),
		handshakeTimeout:           c.handshakeTimeout,
		minVersion:                 c.minVersion,
		maxVersion:                 c.maxVersion,
		insecure:                   c.insecure,
		anchorOnly:                 c.anchorOnly,
		certificatePublicKeySHA256: append([][]byte(nil), c.certificatePublicKeySHA256...),
		timeFunc:                   c.timeFunc,
		store:                      c.store,
	}
}

func newSystemTLSConfig(ctx context.Context, serverAddress string, options option.OutboundTLSOptions, allowEmptyServerName bool, engineName string) (systemTLSConfig, SystemTLSValidated, error) {
	validated, err := ValidateSystemTLSOptions(ctx, options, engineName)
	if err != nil {
		return systemTLSConfig{}, SystemTLSValidated{}, err
	}
	var serverName string
	if options.ServerName != "" {
		serverName = options.ServerName
	} else if serverAddress != "" {
		serverName = serverAddress
	}
	if serverName == "" && !options.Insecure && !allowEmptyServerName {
		return systemTLSConfig{}, SystemTLSValidated{}, errMissingServerName
	}
	handshakeTimeout := C.TCPTimeout
	if options.HandshakeTimeout > 0 {
		handshakeTimeout = options.HandshakeTimeout.Build()
	}
	return systemTLSConfig{
		serverName:                 serverName,
		nextProtos:                 append([]string(nil), options.ALPN...),
		handshakeTimeout:           handshakeTimeout,
		minVersion:                 validated.MinVersion,
		maxVersion:                 validated.MaxVersion,
		insecure:                   options.Insecure || len(options.CertificatePublicKeySHA256) > 0,
		anchorOnly:                 validated.Exclusive,
		certificatePublicKeySHA256: append([][]byte(nil), options.CertificatePublicKeySHA256...),
		timeFunc:                   ntp.TimeFuncFromContext(ctx),
		store:                      validated.Store,
	}, validated, nil
}
