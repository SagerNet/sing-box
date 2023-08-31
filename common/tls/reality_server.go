//go:build with_reality_server

package tls

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"net"
	"time"

	"github.com/sagernet/reality"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/debug"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
)

var _ ServerConfigCompat = (*RealityServerConfig)(nil)

type RealityServerConfig struct {
	config *reality.Config
}

func NewRealityServer(ctx context.Context, logger log.Logger, options option.InboundTLSOptions) (*RealityServerConfig, error) {
	var tlsConfig reality.Config

	if options.ACME != nil && len(options.ACME.Domain) > 0 {
		return nil, E.New("acme is unavailable in reality")
	}
	tlsConfig.Time = ntp.TimeFuncFromContext(ctx)
	if options.ServerName != "" {
		tlsConfig.ServerName = options.ServerName
	}
	if len(options.ALPN) > 0 {
		tlsConfig.NextProtos = append(tlsConfig.NextProtos, options.ALPN...)
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
	if len(options.Certificate) > 0 || options.CertificatePath != "" {
		return nil, E.New("certificate is unavailable in reality")
	}
	if len(options.Key) > 0 || options.KeyPath != "" {
		return nil, E.New("key is unavailable in reality")
	}

	tlsConfig.SessionTicketsDisabled = true
	tlsConfig.Type = N.NetworkTCP
	tlsConfig.Dest = options.Reality.Handshake.ServerOptions.Build().String()

	tlsConfig.ServerNames = map[string]bool{options.ServerName: true}
	privateKey, err := base64.RawURLEncoding.DecodeString(options.Reality.PrivateKey)
	if err != nil {
		return nil, E.Cause(err, "decode private key")
	}
	if len(privateKey) != 32 {
		return nil, E.New("invalid private key")
	}
	tlsConfig.PrivateKey = privateKey
	tlsConfig.MaxTimeDiff = time.Duration(options.Reality.MaxTimeDifference)

	tlsConfig.ShortIds = make(map[[8]byte]bool)
	for i, shortIDString := range options.Reality.ShortID {
		var shortID [8]byte
		decodedLen, err := hex.Decode(shortID[:], []byte(shortIDString))
		if err != nil {
			return nil, E.Cause(err, "decode short_id[", i, "]: ", shortIDString)
		}
		if decodedLen > 8 {
			return nil, E.New("invalid short_id[", i, "]: ", shortIDString)
		}
		tlsConfig.ShortIds[shortID] = true
	}

	handshakeDialer, err := dialer.New(adapter.RouterFromContext(ctx), options.Reality.Handshake.DialerOptions)
	if err != nil {
		return nil, err
	}
	tlsConfig.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return handshakeDialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
	}

	if debug.Enabled {
		tlsConfig.Show = true
	}

	return &RealityServerConfig{&tlsConfig}, nil
}

func (c *RealityServerConfig) ServerName() string {
	return c.config.ServerName
}

func (c *RealityServerConfig) SetServerName(serverName string) {
	c.config.ServerName = serverName
}

func (c *RealityServerConfig) NextProtos() []string {
	return c.config.NextProtos
}

func (c *RealityServerConfig) SetNextProtos(nextProto []string) {
	c.config.NextProtos = nextProto
}

func (c *RealityServerConfig) Config() (*tls.Config, error) {
	return nil, E.New("unsupported usage for reality")
}

func (c *RealityServerConfig) Client(conn net.Conn) (Conn, error) {
	return ClientHandshake(context.Background(), conn, c)
}

func (c *RealityServerConfig) Start() error {
	return nil
}

func (c *RealityServerConfig) Close() error {
	return nil
}

func (c *RealityServerConfig) Server(conn net.Conn) (Conn, error) {
	return ServerHandshake(context.Background(), conn, c)
}

func (c *RealityServerConfig) ServerHandshake(ctx context.Context, conn net.Conn) (Conn, error) {
	tlsConn, err := reality.Server(ctx, conn, c.config)
	if err != nil {
		return nil, err
	}
	return &realityConnWrapper{Conn: tlsConn}, nil
}

func (c *RealityServerConfig) Clone() Config {
	return &RealityServerConfig{
		config: c.config.Clone(),
	}
}

var _ Conn = (*realityConnWrapper)(nil)

type realityConnWrapper struct {
	*reality.Conn
}

func (c *realityConnWrapper) ConnectionState() ConnectionState {
	state := c.Conn.ConnectionState()
	return tls.ConnectionState{
		Version:                     state.Version,
		HandshakeComplete:           state.HandshakeComplete,
		DidResume:                   state.DidResume,
		CipherSuite:                 state.CipherSuite,
		NegotiatedProtocol:          state.NegotiatedProtocol,
		NegotiatedProtocolIsMutual:  state.NegotiatedProtocolIsMutual,
		ServerName:                  state.ServerName,
		PeerCertificates:            state.PeerCertificates,
		VerifiedChains:              state.VerifiedChains,
		SignedCertificateTimestamps: state.SignedCertificateTimestamps,
		OCSPResponse:                state.OCSPResponse,
		TLSUnique:                   state.TLSUnique,
	}
}

func (c *realityConnWrapper) Upstream() any {
	return c.Conn
}
